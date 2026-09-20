## Common collector config
```yaml
child:
    type: <type> # type of element
    value: <value> # value to match
    child: <recursive> # for further filtering on elements
    attr: <attribute> # a filter to obtain a value from the current element
```

```yaml
collectors: # list of collectors
    - name: <name> # name of data source
      url: <url> # url of webpage
      container:
          type: <type> # the top level container to find
          value: <value> # the container match value
          child: # recursive to further filter for target <optional>
              type: <type>
              value: <value>
      data: # content data source
          title:
              type: <type> # the element to collect the title from
          href:
              type: <type> # the element to collect the title from
          timestamp:
              type: <type> # the element to collect the title from
              attr: <attribute> # the attribute of the element to collect the title from
```