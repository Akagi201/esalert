# esalert

[![Build Status](https://travis-ci.org/Akagi201/esalert.svg)](https://travis-ci.org/Akagi201/esalert) [![Coverage Status](https://coveralls.io/repos/github/Akagi201/esalert/badge.svg?branch=master)](https://coveralls.io/github/Akagi201/esalert?branch=master) [![Go Report Card](https://goreportcard.com/badge/github.com/Akagi201/esalert)](https://goreportcard.com/report/github.com/Akagi201/esalert) [![GoDoc](https://godoc.org/github.com/Akagi201/esalert?status.svg)](https://godoc.org/github.com/Akagi201/esalert)

Package esalert a simple framework for real-time alerts on data in Elasticsearch.

## Runtime config
* Esalert's runtime configs.
* Configs can be passed from command-line, environment or config file.

## Alert config
* Alert configs contain all the data processing which should be performed.
* Esalert runs with one or more alerts defined in its configuration, each one operating independant of the others.
* Alert configs can be in one file or a directory of files.
* Alert configs use yaml format. Each file contains an array of alerts.

### Alert rule file(s)

```
# esalert.yml
- name: alert_foo
  # other alert parameters

- name: alert_bar
  # other alert parameters
```

OR

```
# esalert.d/foo.yml
- name: alert_foo
  # other alert parameters

- name: alert_foo2
  # other alert parameters
```

and

```
# esalert.d/bar.yml
- name: alert_bar
  # other alert parameters

- name: alert_bar2
  # other alert parameters
```

### Alert document

A single alert has the following fields in its document (all are required):

```
- name: something_unique
  interval: "*/5 * * * * *"
  search_index: # see the search subsection
  search_type:  # see the search subsection
  search:       # see the search subsection
  process:      # see the process subsection
```

#### name

This is an arbitrary string to identify the alert. It must be unique amongst all of the defined alerts.

#### interval

A [jobber-style](https://github.com/Akagi201/utilgo/tree/master/jobber) interval string describing when the search should be run and have the process run on the results.

#### search

The search which should be performed against elasticsearch. The results are simply held onto for the process step, nothing else is done with them at this point.

```
search_index: filebeat-{{.Format "2006.01.02"}}
search_type: logs
# conveniently, json is valid yaml
search: {
        "query": {
            "query_string": {
                "query":"severity:fatal"
            }
        }
}
```

* See [query dsl](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html) docs for more on how to formulate query objects.
* See [query string](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-query-string-query.html#query-string-syntax) docs for more on how to formulate query strings.
* All three fields(`search_index`, `search_type` and `search`) can have go templating applied.
* See the alert context subsection for more information on what fields/methods are available to use.

#### process

Once the search is performed the results are kept in the context, which is then passed into this step. The process lua script then checks these results against whatever conditions are desired, and may optionally return a list of actions to take. See the alert context section for all available fields in ctx.

```
process:
    lua_file: ./foo-process.yml
```

OR

```
process:
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

##### actions

The table returned by process is a list of actions which should be taken. Each action has a type and subsequent fields based on that type.

##### log

Simply logs an INFO message to the console. Useful if you're testing an alert and don't want to set up any real actions yet.

```
{
    type = "log",
    message = "Performing action for alert " .. ctx.Name,
}
```

##### http

Create and execute an http command. A warning is logged if anything except a 2xx response code is returned.

```
{
    type = "http",
    method = "POST", -- optional, defaults to GET
    url = "http://example.com/some/endpoint?ARG1=foo",
    headers = { -- optional
        "X-FOO" = "something",
    },
    body = "some body for " .. ctx.Name, -- optional
}
```

##### slack

Triggers an event in slack. The --slack-key param must be set in the runtime configuration in order to use this action type.

```
{
    type = "slack",
    text = "some text"
}
```

## Alert context

Through its lifecycle each alert has a context object attached to it. The results from the search step are included in it, as well as other data. Here is a description of the available data in the context, as well as how to use it.

NOTE THAT THE CONTEXT IS READ-ONLY IN ALL CASES

### Context fields

```
{
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
}
```

### In lua

Within lua scripts the context is made available as a global variable called `ctx`. Fields on it are directly addressable using the above names, for example `ctx.HitCount` and `ctx.Hits[1].ID`.

### In go template

In some areas go templates, provided by the template/text package, are used to add some dynamic capabilities to otherwise static configuration fields. In these places the context is made available as the root object. For example, {{.HitCount}}.

In addition to the fields defined above, the root template object also has some methods on it which may be helpful for working with dates. All methods defined on go's time.Time object are available. For example, to format a string into the filebeat index for the current day:

```
filebeat-{{.Format "2006.01.02"}}
```

And to do the same, but for yesterday:

```
filebeat-{{(.AddDate 0 0 -1).Format "2006.01.02"}}
```
