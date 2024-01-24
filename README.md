# tmplreload - Auto-Reloading Templates for Go

`tmplreload` is a Go module that provides auto-reloading templates based on the `html/template` package. It allows you to create templates that automatically reload when the underlying template file changes. This can be particularly useful during development when you want to see the changes to your templates without restarting your application.

## Features

* **Auto-Reloading:** Templates are automatically reloaded when the underlying file changes.
* **Function Map Management:** Easily add, remove, or update template functions.
* **Directory Watching:** Watch multiple directories for changes and automatically reload templates.

## Installation

To use `tmplreload` in your Go project, you can simply run:

```bash
go get github.com/NIR3X/tmplreload
```

## Example Usage

```go
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/NIR3X/tmplreload"
)

func main() {
	// Create a new TmplColl (Template Collection).
	tmplColl := tmplreload.New()

	// Define a function to be used in the template.
	funcMap := template.FuncMap{
		"currentTime": func() string {
			return time.Now().Format(time.RFC3339)
		},
	}

	// Add the function to the TmplColl function map.
	tmplColl.FuncsAdd(funcMap)

	// Parse template files in the "templates" directory.
	err := tmplColl.ParseGlob("templates/*.html")
	if err != nil {
		fmt.Println("Error parsing templates:", err)
		return
	}

	// Start an HTTP server to render the templates.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Execute the template and pass data.
		data := struct{ Message string }{"Hello, tmplreload!"}
		err := tmplColl.ExecuteTemplate(w, "templates/index.html", data)
		if err != nil {
			http.Error(w, "Error rendering template", http.StatusInternalServerError)
		}
	})

	// Start the server on port 8000.
	fmt.Println("Server is running on http://localhost:8000")
	http.ListenAndServe(":8000", nil)
}
```

## Example HTML Template (templates/index.html)

```html
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Example Template</title>
</head>
<body>
	<h1>{{ .Message }}!</h1>
	<p>Current time: {{ currentTime }}</p>
</body>
</html>
```

In this example, the `tmplreload` module is used to create a template collection (`TmplColl`). Templates are parsed from the "templates" directory, and a function (`currentTime`) is added to the function map. The HTTP server renders the template on incoming requests.
