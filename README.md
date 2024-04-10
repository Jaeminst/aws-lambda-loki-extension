# Example Logs API Extension in Go

The provided code sample demonstrates how to get a basic Logs API extension written in Go up and running.

> This is a simple example extension to help you start investigating the Lambda Runtime Logs API. This example code is not production ready. Use it with your own discretion after testing thoroughly.

This sample extension: 
* Subscribes to recieve platform and function logs
* Runs with a main and a helper goroutine: The main goroutine registers to ExtensionAPI and process its invoke and shutdown events (see nextEvent call). The helper goroutine:
    - starts a local HTTP server at the provided port (default 1234) that receives requests from Logs API
    - puts the logs in a synchronized queue (Producer) to be processed by the main goroutine (Consumer)
* Writes the received logs to an S3 Bucket

## Compile package and dependencies

To run this example, you will need to ensure that your build architecture matches that of the Lambda execution environment by compiling with `GOOS=linux` and `GOARCH=amd64` if you are not running in a Linux environment.

Building and saving package into a `bin/extensions` directory:
```bash
$ cd go-example-logs-api-extension
$ GOOS=linux GOARCH=amd64 go build -o bin/extensions/go-example-logs-api-extension main.go
$ chmod +x bin/extensions/go-example-logs-api-extension
```

## Layer Setup Process
The extensions .zip file should contain a root directory called `extensions/`, where the extension executables are located. In this sample project we must include the `go-example-logs-api-extension` binary.

Creating zip package for the extension:
```bash
$ cd bin
$ zip -r extension.zip extensions/
```

Publish a new layer using the `extension.zip` and capture the produced layer arn in `layer_arn`. If you don't have jq command installed, you can run only the aws cli part and manually pass the layer arn to `aws lambda update-function-configuration`.
```bash
layer_arn=$(aws lambda publish-layer-version --layer-name "go-example-logs-api-extension" --region "<use your region>" --zip-file  "fileb://extension.zip" | jq -r '.LayerVersionArn')
```

Add the newly created layer version to a Lambda function.
```bash
aws lambda update-function-configuration --region <use your region> --function-name <your function name> --layers $layer_arn
```

## Function Invocation and Extension Execution

Configure the extension by setting below environment variables

* `LOKI_PUSH_URL` - This is the URL to your Loki instance. It should include the scheme (http or https), the hostname, and the port number if applicable. Do not include the API endpoint path (/loki/api/v1/push) in this URL; the extension will automatically append the necessary path to this base URL.
    > Example: `http://localhost:3100`

* `LOKI_AUTH_TOKEN` - The authentication token required for pushing logs to your Loki instance. This token is used to authenticate the requests made from the extension to the Loki server. Depending on your Loki setup, this might be a "Bearer" token or another form of API key.
    > Example: `eyJhbGciOiJIUzI1NiIsInR5cCIgOiAi...`
