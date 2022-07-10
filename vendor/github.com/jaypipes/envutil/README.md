# `envutil` - A utility library for environment variables [![Build Status](https://travis-ci.org/jaypipes/envutil.svg?branch=master)](https://travis-ci.org/jaypipes/envutil)

`envutil` is a tiny Golang library with some utility functions for dealing with
environment variables. It basically exists because I kept copying the same
utility code into my Golang libraries and applications and wanted to lib-ify
this stuff instead of duplicating it.

## WithDefault Functions

There are a number of functions named `WithDefault{Type}()` which accept the
key of an environment variable and a default value. The function checks to see
if the environs contains the key and, if not, returns the default value. If the
environs *does* contain the key, then the value of the environment variable
with that key is transformed from a string to the `{Type}` mentioned in the
function name.

The functions and their signatures are as follows:

* `WithDefault(key string, def string) string`: If `key` isn't found in the
  environ, returns `def`

* `WithDefaultInt(key string, def int) int`: If `key` isn't found in the
  environ, returns `def`. If `key` is found, converts the environment variable
  value to an `int`. If the conversion fails, returns `def`.

* `WithDefaultBool(key string, def bool) bool`: If `key` isn't found in the
  environ, returns `def`. If `key` is found, converts the environment variable
  value to an `bool`. If the conversion fails, returns `def`.


```go
package main

import (
    "fmt"
    "os"

    "github.com/jaypipes/envutil"
)

func main() {
    fmt.Println("Testing envutil...")
    val, ok := os.LookupEnv("SOME_RANDOM_ENV_KEY")
    if !ok {
        fmt.Println(" SOME_RANDOM_ENV_KEY does not exist in environs.")
    } else {
        fmt.Printf(" SOME_RANDOM_ENV_KEY exists in environs with value %s\n", val)
    }
    strval := envutil.WithDefault("SOME_RANDOM_ENV_KEY", "my default str")
    fmt.Printf(" WithDefault(\"SOME_RANDOM_ENV_KEY\", \"my default str\") returned '%s'\n", strval)
    intval := envutil.WithDefaultInt("SOME_RANDOM_ENV_KEY", 42)
    fmt.Printf(" WithDefaultInt(\"SOME_RANDOM_ENV_KEY\", 42) returned %d\n", intval)

    fmt.Println(" Setting SOME_RANDOM_ENV_KEY to 'random str'.")
    os.Setenv("SOME_RANDOM_ENV_KEY", "random str")
    defer os.Unsetenv("SOME_RANDOM_ENV_KEY")
    intval = envutil.WithDefaultInt("SOME_RANDOM_ENV_KEY", 42)
    fmt.Printf(" WithDefaultInt(\"SOME_RANDOM_ENV_KEY\", 42) returned %d\n", intval)

    fmt.Println(" Setting SOME_RANDOM_ENV_KEY to '12'.")
    os.Setenv("SOME_RANDOM_ENV_KEY", "12")
    intval = envutil.WithDefaultInt("SOME_RANDOM_ENV_KEY", 42)
    fmt.Printf(" WithDefaultInt(\"SOME_RANDOM_ENV_KEY\", 42) returned %d\n", intval)

    fmt.Println(" Setting SOME_RANDOM_ENV_KEY to 'falsy'.")
    os.Setenv("SOME_RANDOM_ENV_KEY", "falsy")
    boolval := envutil.WithDefaultBool("SOME_RANDOM_ENV_KEY", true)
    fmt.Printf(" WithDefaultBool(\"SOME_RANDOM_ENV_KEY\", true) returned %v\n", boolval)

    fmt.Println(" Setting SOME_RANDOM_ENV_KEY to 'false'.")
    os.Setenv("SOME_RANDOM_ENV_KEY", "false")
    boolval = envutil.WithDefaultBool("SOME_RANDOM_ENV_KEY", true)
    fmt.Printf(" WithDefaultBool(\"SOME_RANDOM_ENV_KEY\", true) returned %v\n", boolval)
}
```

## Developers

Contributions to `envutil` are welcomed! Fork the repo on GitHub and submit a pull
request with your proposed changes. Or, feel free to log an issue for a feature
request or bug report.
