var child_process = require('child_process'),
    request = require('request'),
    AWS = require('aws-sdk'),
    moment = require('moment'),
    Q = require('q');

require('toml-require').install();

var aaa_cmd_path = './main';
var executorFunctionName = 'aaa-executor';

var slack_incoming_webhook_url;
var kms = new AWS.KMS();
var lambda = new AWS.Lambda();

/*
 * Configuration
 */
var config = require('./aaa_lambda.toml');

exports.handler = function(event, context) {
  var encrypted_buf = new Buffer(
      config.schedular.encrypted_slack_incoming_webhook_url, 'base64');
  var cipher_text = {CiphertextBlob: encrypted_buf};

  kms.decrypt(cipher_text, function (err, data) {
    if (err) {
      console.log('decrypt error: ' + err);
      context.fail(err);
    } else {
      slack_incoming_webhook_url = data.Plaintext.toString('ascii');
      process_event(event, context);
    }
  });
};

var process_event = function(event, context) {
  var candidates = [];

  do_aaa_ls()
  .then(function(json_buf) {
    console.log('aaa ls ok: ' + json_buf);
    var domains = JSON.parse(json_buf);

    domains.forEach(function(domain) {
      var after_1m = moment().add(config.schedular.cert_renewal_days_before, "days");
      var after_40d = moment().add(config.schedular.authz_renewal_days_before, "days");

      var cert_not_after = moment(domain.certificate.not_after);
      var authz_expires = moment(domain.authorization.expires);

      var candidate = {
        'email': domain.email,
        'domains': [domain.domain],
        'renewal': true,
        'event': {
          'response_url': slack_incoming_webhook_url,
        }
      };

      domain.certificate.san.forEach(function(domain) {
        if (!is_domain_in(domain, candidate.domains)) {
          candidate.domains.push(domain);
        }
      });

      if (after_40d.isAfter(authz_expires)) {
        candidate.command = 'authz';
      } else if (after_1m.isAfter(cert_not_after)) {
        candidate.command = 'cert';
      }

      if (candidate.command) {
        candidates.push(candidate);
      }
    });

    console.log('slack incoming webhook url: ' + slack_incoming_webhook_url);
    console.log('candidates: ' + JSON.stringify(candidates));

    if (candidates.length === 0) {
      post_slack('checking certificates renewal but no certificates found to be renewal')
      .then(function(response) {
        context.succeed('slack resp: ' + JSON.stringify(response));
      })
      .fail(function(response) {
        context.fail(JSON.stringify(response));
      });
    } else {
      // firing renewal event for each certificate
      var promises = [];
      candidates.forEach(function(candidate) {
        var deferred = Q.defer();
        console.log('invoking: ' + JSON.stringify(candidate));
        lambda.invoke({
          'FunctionName': executorFunctionName,
          'InvocationType': 'Event',
          'Payload': JSON.stringify(candidate),
        }, function(error) {
          if (error) {
            console.log('failed to invoke executore: ' + error);
            deferred.reject(error);
          } else {
            deferred.resolve();
          }
        });
        promises.push(deferred.promise);
      });

      return Q.all(promises);
    }
  })
  .then(function() {
    var text;
    if (candidates.length === 0) {
      text = 'checking certificates renewal but no certificates found to be renewal';
    } else {
      text = "renewal processes have been invoked\n" +
        "```\n" +
        JSON.stringify(candidates, null, 2) +
        "\n```";
    }

    post_slack(text)
    .then(function() {
      context.succeed();
    });
  })
  .fail(function(buf) {
    console.log('failed to invoke lambda functions');
    post_slack("```\n" + buf + "\n```")
    context.fail(buf);
  })
  .done();
};

var do_aaa_ls = function() {
  var deferred = Q.defer();

  var cmd = [
    aaa_cmd_path, 'ls',
    '--s3-bucket', config.executor.s3_bucket,
    '--s3-kms-key', config.executor.kms_key,
  ];

  var json_buf = '';
  var log_buf = '';
  var child = child_process.exec(cmd.join(' '));

  child.stdout.on('data', function(data) {
    console.log(data);
    json_buf += data;
  });
  child.stderr.on('data', function(data) {
    console.log(data);
    log_buf += data;
  });

  child.on('exit', function(code) {
    if (code !== 0) {
      console.log('aaa exited with non-zero code: '+code);
      deferred.reject(log_buf);
    } else {
      console.log('aaa exited successuflly');
      deferred.resolve(json_buf);
    }
  });

  return deferred.promise;
};

var post_slack = function(text) {
  var deferred = Q.defer();

  request.post(slack_incoming_webhook_url, {
    'json': true,
    'body': {
      'text': text,
    },
    'headers': {
      'Content-Type': 'application/json',
    },
  }, function(error, response) {
    if (response.statusCode !== 200) {
      deferred.reject(response);
    } else {
      deferred.resolve(response);
    }
  });

  return deferred.promise;
};

var is_domain_in = function(domain, domains) {
  for (var i = 0; i < domains.length; i++) {
    if (domain === domains[i]) {
      return true;
    }
  }

  return false;
};
