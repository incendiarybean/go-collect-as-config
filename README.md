# Basic Webpage Article Collector

## Configuration

Create/update a file named `collectors.yml` in the `config` directory.

Each collector starts with the same config:
```yaml
collectors: # Root of the YAML (required)

    # Collector Entry
    - name: <name> # name of data source
      url: <url> # url of webpage
      container: <child> # filters for stripping the articles from HTML
      data: 
        title: <child> # filters for stripping the title from HTML
        href: <child> # filters for stripping the link from HTML
        img: <child> # filters for stripping the image from HTML
        timestamp: <child> # filters for stripping the timestamp from HTML
```

It includes a `name` of the webpage, can be anything.
It includes a `url` of the webpage, *must* be the URL of the webpage you want to collect.
It includes a `container` which *must* be defined using the child filters (recursively) to find the articles.
It contains a `data` field, which contains `title` `href` `img` `timestamp` fields which are all child filters to extract the fields from the HTML.

Each child filter is defined by a `child` type:
```yaml
type: <type> # the type of element to find, e.g. a, img, section, article
attr: <attribute> # a filter to obtain a value from the current element by attribute
value: <value> # a filter to obtain a value from the current element by attrubute value (attribute must be supplied)
child: <recursive> # for defining deeper searches
```

You can use children recursively to obtain deepor values, or provide a more defined scope for value:

```yaml
title:
    attr: role
    value: text
    child:
        type: span
```
> This would find an element with the attribute `role` that is equal to `text` and then filter again on the child elements for a type of `span`.

## Usage

1. Create or update the `config/collectors.yml`.
2. Run the main file:
   ```bash
   go run main.go
   ```
3. View the content on `http://localhost:8080`.

> All content is stored locally within an SQLite DB

Content is parsed in the JSON shape:
```json
{
    "Id": "<Sha1 Hash>",
    "Outlet": "<Name of outlet>",
    "Title": "<Article title>",
    "Img": "<Article src>",
    "Href": "<Article link>",
    "Timestamp": "<Article Timestamp>"
}
```