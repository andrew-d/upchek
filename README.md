# upchek

upchek is a simple tool to run healthcheck scripts in a user-provided directory
and show the results in a simple, plain HTML web page.

It makes it easy to add ad-hoc healthchecks to your system and have them
automatically run and displayed.

It also exposes a `/healthz` endpoint that can be used to check the overall
system health.

## Usage

upchek has the following command line options:

```
Usage of upchek:
  -d, --directory string     directory for healthcheck scripts (default "/etc/upchek")
  -l, --listen string        address to listen on (default ":8080")
      --remote stringArray   list of other upchek instances to aggregate results from
  -v, --verbose              verbose output
```

The `--directory` flag is used to specify the directory where upchek will look
for healthcheck scripts. The scripts should be executable and should output
plain text to stdout and/or stderr. The scripts should exit with a status code
of 0 if the healthcheck succeeded, and a non-zero status code if it failed.

The `--remote` flag is used to specify other instances of upchek, and it can be
specified multiple times. For each specified instance, upchek will fetch the
(non-aggreated) healthcheck results from that instance and display them in the
web interface. Note that the results from another instance do not affect the
`/healthz` endpoint for the current instance, nor will they be recursively
fetched by other instances scraping this one.

## Screenshots

![full size](docs/upchek-desktop.png)
![mobile](docs/upchek-mobile.png)
