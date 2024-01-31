package tmplreload

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"
)

func TestTmplColl(t *testing.T) {
	// Create a temporary directory.
	tmpDir, err := os.MkdirTemp("", "tmplreload")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testData := `<html><head><title>Test</title></head><body><h1>{{ TestMsg }}</h1></body></html>`
	testDataExpected := "<html><head><title>Test</title></head><body><h1>Test</h1></body></html>"
	updatedTestData := "<html><head><title>Test</title></head><body><h1>{{ UpdatedTestMsg }}</h1></body></html>"
	updatedTestDataExpected := "<html><head><title>Test</title></head><body><h1>UpdatedTest</h1></body></html>"

	// Create an HTML file.
	filePath := filepath.Join(tmpDir, "test.html")
	htmlFile, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer htmlFile.Close()
	_, err = htmlFile.WriteString(testData)
	if err != nil {
		t.Fatal(err)
	}
	err = htmlFile.Sync()
	if err != nil {
		t.Fatal(err)
	}

	// Create a TmplColl.
	tmplColl := NewTmplColl()
	defer tmplColl.Close()
	tmplColl.FuncsAdd(template.FuncMap{
		"TestMsg": func() string {
			return "Test"
		},
	})
	err = tmplColl.ParseFiles(filePath)
	if err != nil {
		t.Fatal(err)
	}

	// Render the template.
	var buf bytes.Buffer
	err = tmplColl.ExecuteTemplate(&buf, filePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != testDataExpected {
		t.Fatal("Unexpected template output")
	}

	// Add a function to the function map.
	tmplColl.FuncsAdd(template.FuncMap{
		"UpdatedTestMsg": func() string {
			return "UpdatedTest"
		},
	})

	// Wait a second for the changes to take effect.
	time.Sleep(time.Second)

	// Update the HTML file.
	_, err = htmlFile.Seek(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = htmlFile.WriteString(updatedTestData)
	if err != nil {
		t.Fatal(err)
	}
	err = htmlFile.Sync()
	if err != nil {
		t.Fatal(err)
	}

	// Render the template again.
	buf.Reset()
	err = tmplColl.ExecuteTemplate(&buf, filePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != updatedTestDataExpected {
		t.Fatal("Unexpected template output")
	}

	// Remove the function from the function map.
	tmplColl.FuncsRemove("TestMsg")

	// Wait a second for the changes to take effect.
	time.Sleep(time.Second)

	// Render the template again.
	buf.Reset()
	err = tmplColl.ExecuteTemplate(&buf, filePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != updatedTestDataExpected {
		t.Fatal("Unexpected template output")
	}

	// Remove the function from the function map.
	tmplColl.FuncsRemove("UpdatedTestMsg")

	// Wait a second for the changes to take effect.
	time.Sleep(time.Second)

	// Render the template again.
	buf.Reset()
	err = tmplColl.ExecuteTemplate(&buf, filePath, nil)
	if err == nil {
		t.Fatal("Expected error")
	}
	if buf.String() == updatedTestDataExpected {
		t.Fatal("Expected template output to be empty")
	}

	// Add a function to the function map.
	tmplColl.FuncAdd("UpdatedTestMsg", func() string {
		return "UpdatedTest"
	})

	// Wait a second for the changes to take effect.
	time.Sleep(time.Second)

	// Render the template again.
	buf.Reset()
	err = tmplColl.ExecuteTemplate(&buf, filePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != updatedTestDataExpected {
		t.Fatal("Unexpected template output")
	}

	// Change the delimiters.
	tmpl := tmplColl.Lookup(filePath)
	tmpl.Delims("[[", "]]")
	// Reload the template to apply the changes.
	err = tmpl.Reload()
	if err != nil {
		t.Fatal(err)
	}

	// Render the template again.
	buf.Reset()
	err = tmplColl.ExecuteTemplate(&buf, filePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != updatedTestData {
		t.Fatal("Unexpected template output")
	}
}
