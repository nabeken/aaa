var child_process = require('child_process'),
    request = require('request'),
    Q = require('q');

require('toml-require').install();

var aaa_cmd_path = './main';

/*
 * Configuration
 */
var config = require('./aaa_lambda.toml');

exports.handler = function(event, context) {
  var message;

  // Retrieve message from SNS
  if (event.Records) {
    message = JSON.parse(event.Records[0].Sns.Message);
  } else {
    // Direct invoke
    message = event;
  }

  console.log('MSG: ' + JSON.stringify(message));

  var cmds = build_aaa_cmds(message);
  console.log('CMDs: ' + cmds.join("\n"));

  var promises = [];
  cmds.forEach(function(cmd) {

    var deferred = Q.defer();

    var myenv = {}
    for (var env in process.env) {
      myenv[env] = process.env[env];
    }
    if (config.executor.directory_url) {
      myenv['AAA_DIRECTORY_URL'] = config.executor.directory_url;
    }

    var child = child_process.exec(cmd, {'env': myenv});

    // Log process stdout and stderr
    var log_buf = '';

    child.stdout.on('data', console.log);
    child.stderr.on('data', function(data) {
      console.error(data);
      log_buf += data;
    });

    child.on('exit', function(code) {
      if (code !== 0) {
        console.log('aaa exited with non-zero code: '+code);
        deferred.reject(log_buf);
      } else {
        console.log('aaa exited successuflly');
        deferred.resolve(log_buf);
      }
    });

    promises.push(deferred.promise);
  });

  Q.all(promises)
  .then(function(results) {
    if (message.event) {
      console.log('aaa event: ' + JSON.stringify(message.event));

      var deferred = Q.defer();

      // send a response to slack
      request.post(message.event.response_url, {
        'json': true,
        'body': build_slack_notification(results, message),
        'headers': {
          'Content-Type': 'application/json',
        },
      }, function(error, response) {
        if (error) {
          deferred.reject(error);
        } else {
          deferred.resolve();
        }
      });

      return deferred.promise;
    }
  })
  .then(function(results) {
    console.log('aaa ok: ' + results);
    context.succeed();
  })
  .fail(function(results) {
    console.log('aaa fail: ' + results);
    if (message.event) {
      var deferred = Q.defer();

      // send a response to slack
      request.post(message.event.response_url, {
        'json': true,
        'body': build_slack_error_notification(results, message),
        'headers': {
          'Content-Type': 'application/json',
        },
      }, function() {
        deferred.resolve();
      });

      return deferred.promise;
    }
  })
  .finally(function(results) {
    // In the context of lambda invoke, we still set this as succeed to avoind retrying.
    context.succeed();
  })
  .done();
};

var build_aaa_cmds = function(message) {
  var cmd = [
    aaa_cmd_path, message.command,
    '--s3-bucket', config.executor.s3_bucket,
    '--s3-kms-key', config.executor.kms_key,
    '--email', message.email,
  ];

  if (message.renewal) {
    cmd.push('--renewal');
  }

  switch (message.command) {
    case 'upload':
      cmd.push('--domain', message.domains[0]);
      return [cmd.join(' ')];

    case 'authz':
      cmd.push('--challenge', config.executor.challenge);

      var cmds = [];
      for (var i = 0; i < message.domains.length; i++) {
        var authz_cmd = [].concat(cmd);
        authz_cmd.push('--domain', message.domains[i]);
        cmds.push(authz_cmd.join(' '));
      }

      return cmds;
    case 'cert':
      common_name = message.domains[0];
      cmd.push('--cn', common_name);

      for (var i = 1; i < message.domains.length; i++) {
        cmd.push('--domain', message.domains[i]);
      }

      return [cmd.join(' ')];
  }

  return '';
};

var build_slack_notification = function(results, message) {
  switch (message.command) {
    case 'upload':
        return {
          'response_type': 'in_channel',
          'text': '@' + message.event.user_name + ": the cert has been uploaded to IAM!\n" +
                  "```\n" +
                  results +
                  '```',
        };
    case 'authz':
      if (message.renewal) {
        return {
          'response_type': 'in_channel',
          'text': 'the authorization for ' + message.domains + ' has been renewed.',
        };
      } else {
        return {
          'response_type': 'in_channel',
          'text': '@' + message.event.user_name + ': the authorization has been done! ' +
                  'You are ready to issue the certificates.',
        };
      }
    case 'cert':
      return {
        'response_type': 'in_channel',
        'text': '@' + message.event.user_name + ': the certificates for ' +
                message.domains.join(', ') + " now available!\n" +
                "```\n" +
                "aws s3 sync s3://" + config.executor.s3_bucket + '/aaa-data/' +
                message.email + '/domain/' + message.domains[0] + " " + message.domains[0] +
                "\n```",
      };
  }
};

var build_slack_error_notification = function(results, message) {
  return {
    'response_type': 'in_channel',
    'text': '`aaa` exits with non-zero code. Logs:' +
            "```\n" +
            results +
            '```',
  };
};
