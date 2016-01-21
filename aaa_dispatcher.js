/*
 * This is based on the blueprint `slack-echo-command' provided by AWS.
 */
var AWS = require('aws-sdk');

require('toml-require').install();

// token will be initialized later.
var token;

var lambda = new AWS.Lambda();

/*
 * Configuration via TOML
 */
var config = require('./aaa_lambda.toml');

exports.handler = function (event, context) {
  if (token) {
    // Container reuse, simply process the event with the key in memory
    processEvent(event, context);
  } else {
    var encryptedBuf = new Buffer(config.dispatcher.encrypted_slack_token, 'base64');
    var cipherText = {CiphertextBlob: encryptedBuf};

    var kms = new AWS.KMS();
    kms.decrypt(cipherText, function (err, data) {
      if (err) {
        console.log('Decrypt error: ' + err);
        context.fail(err);
      } else {
        token = data.Plaintext.toString('ascii');
        processEvent(event, context);
      }
    });
  }
};

var processEvent = function(event, context) {
  var requestToken = event.token;
  if (requestToken !== token) {
    console.error('Request token (' + requestToken + ') does not match exptected');
    context.fail('Invalid request token');
  }

  for (var i in event) {
    event[i] = decodeURIComponent(event[i].replace(/\+/g, '%20'));
  }

  var user = event.user_name;
  var command = event.command;
  var channel = event.channel_name;
  var commandText = event.text;

  console.log('text: ' + commandText);

  var commandArgs = commandText.split(' ');

  if (commandArgs.length < 3) {
    context.fail('invalid command args received: ' + commandArgs);
    return;
  }

  var aaaCommand = commandArgs.shift();
  var aaaEmail = commandArgs.shift();
  var aaaDomains = commandArgs;

  var aaaReq = {
    'command': aaaCommand,
    'email': aaaEmail,
    'domains': aaaDomains,
    'event': event,
  };

  console.log('aaa-request: ' + JSON.stringify(aaaReq));

  lambda.invoke({
    'FunctionName': config.lambda.executor_function_name,
    'InvocationType': 'Event',
    'Payload': JSON.stringify(aaaReq),
  }, function(error) {
    if (error) {
      context.fail(JSON.stringify(resp));
    }

    var resp = {
      'response_type': 'in_channel',
      'text': "@" + user + ": request accepted. Please wait a moment...",
    };

    context.succeed(resp);
  });
};
