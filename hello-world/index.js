'use strict';

exports.handler = async (event, context) => {
  console.log('Inside Lambda function');

  return {
    statusCode: 200,
    body: JSON.stringify({
      message: 'hello world',
    }),
  };
};
