

Package esalert a simple framework for real-time alerts on data in Elasticsearch.

## Runtime configuration
esalert's runtime parameters are specified either on the command-line, in the
environment, or in a configuration file. These parameters will include things
like the elasticsearch address, api keys for slack, etc...

## Alert configuration
Another configuration file (or set of configuration files) is also used, this
one being required and containing all data processing which should be performed.
A single "alert" encompasses all of the following:

* A time interval to perform the alert check at
* A search to be performed against elasticsearch
* A lua script which processes the search results and decides what actions to


```go
take
```

esalert runs with one or more alerts defined in its configuration, each one
operating independant of the others.

### The file(s)

Alert configuration is defined in one or more yaml files. Each file contains an
array of alerts, like so

```yaml
# esalert.yml
- name: alert_foo


```go
# other alert parameters
```

- name: alert_bar


```go
# other alert parameters
```

```

`> esalert -a esalert.yaml`

**OR** they could be defined in separate files within the same directory, with
that directory being passed into esalert:

```yaml
# esalert.d/foo.yml
- name: alert_foo


```go
# other alert parameters
```

- name: alert_foo2


```go
# other alert parameters
```

```

and

```yaml
# esalert.d/bar.yml
- name: alert_bar


```go
# other alert parameters
```

- name: alert_bar2


```go
# other alert parameters
```

```

`> esalert -a esalert.d`

### Alert document

A single alert has the following fields in its document (all are required):

```yaml
- name: something_unique


```go
interval: "5 * * * *"
search_index: # see the search subsection
search_type:  # see the search subsection
search:       # see the search subsection
process:      # see the process subsection
```

```

#### name

This is an arbitrary string to identify the alert. It must be unique amongst all
of the defined alerts.

#### interval

A cron-style interval string describing when the search should be run and have
the process run on the results.

#### search

The search which should be performed against elasticsearch. The results are
simply held onto for the process step, nothing else is done with them at this
point.

```yaml
search_index: filebeat-{{.Format "2006.01.02"}}
search_type: logs
# conveniently, json is valid yaml
search: {


```go
"query": {
    "query_string": {
        "query":"severity:fatal"
    }
}
```

}
```

See the [query dsl][querydsl] docs for more on how to formulate query objects.
See the [query string][querystring] docs for more on how to formulate query
strings.

In the above examples you can see that the `search_index` fields uses a go
template to generate a date specific index. All three fields (`search_index`,
`search_type`, and `search`) can have go templating applied. See the alert
context subsection for more information on what fields/methods are available to
use.

#### process

Once the search is performed the results are kept in the context, which is then
passed into this step. The process lua script then checks these results against
whatever conditions are desired, and may optionally return a list of actions to
take. See the alert context section for all available fields in `ctx`.

```yaml
process:


```go
lua_file: ./foo-process.yml
```

```

**OR**

```yaml
process:


```go
lua_inline: |
    if ctx.HitCount > 10 then
        return {
            {
                type = "log",
                message = "got " .. ctx.HitCount .. " hits",
            }
        }
    end
    -- To indicate no actions, you can return an empty table, nil, or simply
    -- don't return at all
    return {}
```

```

##### actions

The table returned by process is a list of actions which should be taken. Each
action has a type and subsequent fields based on that type.

###### log

Simply logs an INFO message to the console. Useful if you're testing an alert
and don't want to set up any real actions yet

```lua
{


```go
type = "log",
message = "Performing action for alert " .. ctx.Name,
```

}
```

###### http

Create and execute an http command. A warning is logged if anything except a 2xx
response code is returned.

Example:

```lua
{


```go
type = "http",
method = "POST", -- optional, defaults to GET
url = "<a href="http://example.com/some/endpoint?ARG1=foo">http://example.com/some/endpoint?ARG1=foo</a>",
headers = { -- optional
    "X-FOO" = "something",
},
body = "some body for " .. ctx.Name, -- optional
```

}
```

###### slack

Triggers an event in slack. The `--slack-key` param must be set in the
runtime configuration in order to use this action type.

Example:

```lua
{


```go
type = "slack",

-- optional, defaults to alert's name, used to de-duplicate triggers on
-- slack's end
incident_key = "something",

-- While it's possible to use templated terms in here, it makes the most
-- sense to have this be a static key and use the details dict for dynamic
-- data
description = "A short message about the error",

-- Optional table of extra contextural data about this alert
details = {
    foo = ctx.Some.Data,
    bar = "baz",
},
```

}
```

Through its lifecycle each alert has a context object attached to it. The
results from the search step are included in it, as well as other data. Here is
a description of the available data in the context, as well as how to use it

**NOTE THAT THE CONTEXT IS READ-ONLY IN ALL CASES**

#### Context fields

```
{


```go
Name      string // The alert's name
StartedTS uint64 // The timestamp the alert started at

// The following are filled in by the search step
TookMS      uint64  // Time search took to complete, in milliseconds
HitCount    uint64  // The total number of documents matched
HitMaxScore float64 // The maximum score of all the documents matched

// Array of actual documents matched. Keep in mind that unless you manually
// define a limit in your search query this will be capped at 10 by
// elasticsearch. Usually HitCount is the important data point anyway
Hits []{
    Index  string  // The index the hit came from
    Type   string  // The type the document is
    ID     string  // The unique id of the document
    Score  float64 // The document's score relative to the query
    Source object  // The actual document
}

// If an aggregation was defined in the search query, the results will be
// set here
Aggregations object
```

}
```

#### In lua

Within lua scripts the context is made available as a global variable called
`ctx`. Fields on it are directly addressable using the above names, for example
`ctx.HitCount` and `ctx.Hits[1].ID`.

#### In go template

In some areas go templates, provided by the `template/text` package, are used to
add some dynamic capabilities to otherwise static configuration fields. In these
places the context is made available as the root object. For example,
`{{.HitCount}}`.

In addition to the fields defined above, the root template object also has some
methods on it which may be helpful for working with dates. All methods defined
on go's [time.Time](<a href="https://golang.org/pkg/time/#Time">https://golang.org/pkg/time/#Time</a>) object are available. For
example, to format a string into the filebeat index for the current day:

```
filebeat-{{.Format "2006.01.02"}}
```

And to do the same, but for yesterday:

```
filebeat-{{(.AddDate 0 0 -1).Format "2006.01.02"}}
```

Go's system of date format strings is a bit unique (aka weird), read more about
it [here](<a href="https://golang.org/pkg/time/#Time.Format">https://golang.org/pkg/time/#Time.Format</a>)

[querydsl]: <a href="https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html">https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html</a>
[querystring]: <a href="https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-query-string-query.html#query-string-syntax">https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-query-string-query.html#query-string-syntax</a>
