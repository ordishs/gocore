# GoCore

GoCore is a library of useful tools that can be used to configure microservice and provide logging.  It includes an embedded web front end for statistics and the ability to modify configuration settings without restarting the microservice itself.

The library is split in to a few components:

* Config which provides a structured approach to settings
* Logger for logging the microservice, including realtime tracing via Regex
* Stats for realtime instrumentation of the microservice
* Utils which contains some useful functionality that is used elsewhere in this library
* Examples


## Config

Config reads settings from 2 different files as well as the environment.  All settings are specified in the form ```key=value```.  The order of precedence for how a setting is defined is:

1. Environment
2. settings_local.conf
3. settings.conf

The ```settings_local.conf``` file is normally stored in the same location as the application.  ```settings.local``` can be stored in the same location, but it is more useful to place this in a parent folder of the application so that some settings can we shared across more than one application.

Gocore offers a number of functions to retrieve settings:

```go
func (c *Configuration) Get(key string, defaultValue ...string) (string, bool) {
func (c *Configuration) GetInt(key string, defaultValue ...int) (int, bool) {
func (c *Configuration) GetBool(key string, defaultValue ...bool) bool {
func (c *Configuration) GetURL(key string, defaultValue ...string) (*url.URL, error, bool) {
```

depending on the type of value you are interested in.  For example:

```gocore.Config().Get("database_url")``` will return a string if the setting is found, or "" if it is not found.  The boolean return can be used to check if the setting existed or not.

```gocore.Config().GetInt("database_port", 5432)``` will return an int if the setting is found, or the provided default (5432) if not.  The boolean return can be used to check if the setting existed or not.

```go
if gocore.Config().GetBool("happy", false) {
  fmt.Println("I'm happy")
} else {
  fmt.Println("I'm sad")
}
```

There is also a concept of SETTINGS_CONTEXT which is set via the environment or defaults to "dev" if not.

The general principle is to keep all application settings organised together as it is easier to see differences when they live adjacent to each other and often the same value is used in all different contexts.

The way context is used is probably best explained with an example.  Let's specify a settings.conf with multiple versions of one setting in it:

```conf
url=http://localhost:8080
url.live=https://www.server.com
url.live.uk=https://www.server.co.uk
```

and we have an application that reads that setting:

```go
package main

import (
	"fmt"

	"github.com/ordishs/gocore"
)

func main() {
	url, _, _ := gocore.Config().GetURL("url")
	fmt.Printf("URL is %v\n", url)
}
```

you will now get a different value depending on how you specify SETTINGS_CONTEXT.

```go run main.go``` returns ```http://localhost:8080```

```SETTINGS_CONTEXT=live go run main.go``` returns ```https://www.server.com```

```SETTINGS_CONTEXT=live.uk go run main.go``` returns ```https://www.server.co.uk```

and

```SETTINGS_CONTEXT=live.es go run main.go``` returns ```https://www.server.com```

because there is not SETTINGS_CONTEXT for live.es, and GoCore will keep trying each 'parent' context until it finds an answer. If we ran:

```SETTINGS_CONTEXT=stage.eu.red go run main.go``` 

we could get the value of  ```http://localhost:8080``` because GoCore would have tried:

1. ```url.stage.eu.red``` - not found
2. ```url.stage.eu``` - not found
3. ```url.stage``` - not found
4. ```url``` - http://localhost:8080

The optional default parameter is useful when you want a sensible value for a setting when it is missing from the settings files and environment:

```go
timeoutMillis, _ := gocore.Config().GetInt("timeout_millis", 30000)
```

GoCore config will read the environment and settings files one time only.  This is done when the first setting  is requested.  It is useful to write the results of all the settings at application startup, so that you can record them in the application logs.  The following example:

```go
package main

import (
	"fmt"

	"github.com/ordishs/gocore"
)

func main() {
	fmt.Printf("STATS\n%s\n-------\n\n", gocore.Config().Stats())
}
```

will output:

```
STATS

SETTINGS_CONTEXT
----------------
Not set (dev)

SETTINGS
--------
url=http://localhost:8080

-------
```



## Logger

There are many logging frameworks available and the GoCore logger is very simple implementation with some useful features.

In the examples folder you will see an example of its use.
