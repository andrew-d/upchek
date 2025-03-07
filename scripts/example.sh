#!/bin/sh

main() {
    echo "This is a simple healthcheck script"
    echo "This is some output to stderr" >&2

    # Return a zero status code to indicate success
    exit 0
}

main "$@"
