package dbus

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

type lowerCaseExport struct{}

func (export lowerCaseExport) foo() (string, *Error) {
	return "bar", nil
}

type fooExport struct {
	message Message
}

func (export *fooExport) Foo(message Message, param string) (string, *Error) {
	export.message = message
	return "foo", nil
}

type barExport struct{}

func (export barExport) Foo(param string) (string, *Error) {
	return "bar", nil
}

type badExport struct{}

func (export badExport) Foo(param string) string {
	return "bar"
}

type errorExport struct {
	message Message
}

func (export *errorExport) Run(message Message, param string) (string, error) {
	export.message = message
	return "pass", nil
}

type noErrorExport struct {
	message Message
}

func (export *noErrorExport) Run(message Message, param string) string {
	export.message = message
	return "cool"
}

// Test typical Export usage.
func TestExport(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	connection.Export(server{}, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")
	subtreeObject := connection.Object(name, "/org/guelfey/DBus/Test/Foo")

	var response int64
	err = object.Call("org.guelfey.DBus.Test.Double", 0, int64(2)).Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Double: %s", err)
	}

	if response != 4 {
		t.Errorf("Response was %d, expected 4", response)
	}

	// Verify that calling a subtree of a regular export does not result in a
	// valid method call.
	err = subtreeObject.Call("org.guelfey.DBus.Test.Double", 0, int64(2)).Store(&response)
	if err == nil {
		t.Error("Expected error due to no object being exported on that path")
	}

	// Now remove export
	connection.Export(nil, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	err = object.Call("org.guelfey.DBus.Test.Double", 0, int64(2)).Store(&response)
	if err == nil {
		t.Error("Expected an error since the export was removed")
	}
}

// Test that Exported handlers can use a go error type.
func TestExport_goerror(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	export := &errorExport{}
	connection.ExportAll(export, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Run", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "pass" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected a valid message to be given to handler")
	}
}

// Test that Exported handlers can have no error.
func TestExport_noerror(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	export := &noErrorExport{}
	connection.ExportAll(export, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Run", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "cool" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected a valid message to be given to handler")
	}
}

// Test that Exported handlers can obtain raw message.
func TestExport_message(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	export := &fooExport{}
	connection.Export(export, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected a valid message to be given to handler")
	}
}

// Test Export with an invalid path.
func TestExport_invalidPath(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	err = connection.Export(nil, "foo", "bar")
	if err == nil {
		t.Error("Expected an error due to exporting with an invalid path")
	}
}

// Test Export with an un-exported method. This should not panic, but rather
// result in an invalid method call.
func TestExport_unexportedMethod(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	connection.Export(lowerCaseExport{}, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	call := object.Call("org.guelfey.DBus.Test.foo", 0)
	err = call.Store(&response)
	if err == nil {
		t.Errorf("Expected an error due to calling unexported method")
	}
}

// Test Export with a method lacking the correct return signature. This should
// result in an invalid method call.
func TestExport_badSignature(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	connection.Export(badExport{}, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	call := object.Call("org.guelfey.DBus.Test.Foo", 0)
	err = call.Store(&response)
	if err == nil {
		t.Errorf("Expected an error due to the method lacking the right signature")
	}
}

// Test typical ExportWithMap usage.
func TestExportWithMap(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	mapping := make(map[string]string)
	mapping["Double"] = "double" // Export this method as lower-case

	connection.ExportWithMap(server{}, mapping, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response int64
	err = object.Call("org.guelfey.DBus.Test.double", 0, int64(2)).Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling double: %s", err)
	}

	if response != 4 {
		t.Errorf("Response was %d, expected 4", response)
	}
}

// Test that ExportWithMap does not export both method alias and method.
func TestExportWithMap_bypassAlias(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	mapping := make(map[string]string)
	mapping["Double"] = "double" // Export this method as lower-case

	connection.ExportWithMap(server{}, mapping, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response int64
	// Call upper-case Double (i.e. the real method, not the alias)
	err = object.Call("org.guelfey.DBus.Test.Double", 0, int64(2)).Store(&response)
	if err == nil {
		t.Error("Expected an error due to calling actual method, not alias")
	}
}

// Test typical ExportSubtree usage.
func TestExportSubtree(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	export := &fooExport{}
	connection.ExportSubtree(export, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")

	// Call a subpath of the exported path
	object := connection.Object(name, "/org/guelfey/DBus/Test/Foo")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected the raw message, got an invalid one")
	}

	// Now remove export
	connection.Export(nil, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err == nil {
		t.Error("Expected an error since the export was removed")
	}
}

// Test that using ExportSubtree with exported methods that don't contain a
// Message still work, they just don't get the message.
func TestExportSubtree_noMessage(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	connection.ExportSubtree(server{}, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")

	// Call a subpath of the exported path
	object := connection.Object(name, "/org/guelfey/DBus/Test/Foo")

	var response int64
	err = object.Call("org.guelfey.DBus.Test.Double", 0, int64(2)).Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Double: %s", err)
	}

	if response != 4 {
		t.Errorf("Response was %d, expected 4", response)
	}

	// Now remove export
	connection.Export(nil, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	err = object.Call("org.guelfey.DBus.Test.Double", 0, int64(2)).Store(&response)
	if err == nil {
		t.Error("Expected an error since the export was removed")
	}
}

// Test that a regular Export takes precedence over ExportSubtree.
func TestExportSubtree_exportPrecedence(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	// Register for the entire subtree of /org/guelfey/DBus/Test
	connection.ExportSubtree(&fooExport{},
		"/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")

	// Explicitly register for /org/guelfey/DBus/Test/Foo, a subpath of above
	connection.Export(&barExport{}, "/org/guelfey/DBus/Test/Foo",
		"org.guelfey.DBus.Test")

	// Call the explicitly exported path
	object := connection.Object(name, "/org/guelfey/DBus/Test/Foo")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "bar" {
		t.Errorf(`Response was %s, expected "bar"`, response)
	}

	response = "" // Reset response so errors aren't confusing

	// Now remove explicit export
	connection.Export(nil, "/org/guelfey/DBus/Test/Foo", "org.guelfey.DBus.Test")
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	// Now the subtree export should handle the call
	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}
}

// Test typical ExportSubtreeWithMap usage.
func TestExportSubtreeWithMap(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	mapping := make(map[string]string)
	mapping["Foo"] = "foo" // Export this method as lower-case

	connection.ExportSubtreeWithMap(&fooExport{}, mapping, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")

	// Call a subpath of the exported path
	object := connection.Object(name, "/org/guelfey/DBus/Test/Foo")

	var response string
	// Call the lower-case method
	err = object.Call("org.guelfey.DBus.Test.foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	// Now remove export
	connection.Export(nil, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	err = object.Call("org.guelfey.DBus.Test.foo", 0, "qux").Store(&response)
	if err == nil {
		t.Error("Expected an error since the export was removed")
	}
}

// Test that ExportSubtreeWithMap does not export both method alias and method.
func TestExportSubtreeWithMap_bypassAlias(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	mapping := make(map[string]string)
	mapping["Foo"] = "foo" // Export this method as lower-case

	connection.ExportSubtreeWithMap(&fooExport{}, mapping, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test/Foo")

	var response string
	// Call upper-case Foo (i.e. the real method, not the alias)
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err == nil {
		t.Error("Expected an error due to calling actual method, not alias")
	}
}

func TestExportMethodTable(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]
	export := &fooExport{}
	tbl := make(map[string]interface{})
	tbl["Foo"] = func(message Message, param string) (string, *Error) {
		return export.Foo(message, param)
	}
	tbl["Foo2"] = export.Foo
	connection.ExportMethodTable(tbl, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")

	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected the raw message, got an invalid one")
	}

	err = object.Call("org.guelfey.DBus.Test.Foo2", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected the raw message, got an invalid one")
	}

	// Now remove export
	connection.Export(nil, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err == nil {
		t.Error("Expected an error since the export was removed")
	}
}

func TestExportSubtreeMethodTable(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	export := &fooExport{}
	tbl := make(map[string]interface{})
	tbl["Foo"] = func(message Message, param string) (string, *Error) {
		return export.Foo(message, param)
	}
	tbl["Foo2"] = export.Foo
	connection.ExportSubtreeMethodTable(tbl, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")

	// Call a subpath of the exported path
	object := connection.Object(name, "/org/guelfey/DBus/Test/Foo")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected the raw message, got an invalid one")
	}

	err = object.Call("org.guelfey.DBus.Test.Foo2", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected the raw message, got an invalid one")
	}

	// Now remove export
	connection.Export(nil, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err == nil {
		t.Error("Expected an error since the export was removed")
	}
}

func TestExportMethodTable_NotFunc(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]
	export := &fooExport{}
	tbl := make(map[string]interface{})
	tbl["Foo"] = func(message Message, param string) (string, *Error) {
		return export.Foo(message, param)
	}
	tbl["Foo2"] = "foobar"

	connection.ExportMethodTable(tbl, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Foo: %s", err)
	}

	if response != "foo" {
		t.Errorf(`Response was %s, expected "foo"`, response)
	}

	if export.message.serial == 0 {
		t.Error("Expected the raw message, got an invalid one")
	}

	err = object.Call("org.guelfey.DBus.Test.Foo2", 0, "qux").Store(&response)
	if err == nil {
		t.Errorf("Expected an error since the Foo2 was not a function")
	}
}

func TestExportMethodTable_ReturnNotError(t *testing.T) {
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]
	export := &fooExport{}
	tbl := make(map[string]interface{})
	tbl["Foo"] = func(message Message, param string) (string, string) {
		out, _ := export.Foo(message, param)
		return out, out
	}

	connection.ExportMethodTable(tbl, "/org/guelfey/DBus/Test", "org.guelfey.DBus.Test")
	object := connection.Object(name, "/org/guelfey/DBus/Test")

	var response string
	err = object.Call("org.guelfey.DBus.Test.Foo", 0, "qux").Store(&response)
	if err == nil {
		t.Errorf("Expected an error since the Foo did not have a final return as *dbus.Error")
	}
}

// Test that introspection works on sub path of every exported object
func TestExportSubPathIntrospection(t *testing.T) {
	const (
		introIntf    = "org.freedesktop.DBus.Introspectable"
		respTmpl     = `^<node>\s*<node\s+name="%s"\s*/>\s*</node>$`
		pathstr      = "/org/guelfey/DBus/Test"
		foopathstr   = pathstr + "/Foo"
		barpathstr   = pathstr + "/Bar"
		test1intfstr = "org.guelfey.DBus.Test1"
		test2intfstr = "org.guelfey.DBus.Test2"
		intro        = `
			<node>
			<interface name="` + test1intfstr + `">
				<method name="Foo">
					<arg direction="out" type="s"/>
				</method>
			</interface>
			<interface name="` + test2intfstr + `">
				<method name="Foo">
					<arg direction="out" type="s"/>
				</method>
				<method name="Bar">
					<arg direction="out" type="s"/>
				</method>
			</interface>
			<interface name="` + introIntf + `">
				<method name="Introspect">
					<arg name="out" direction="out" type="s"/>
				</method>
			</interface>
			</node>`
	)
	connection, err := ConnectSessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}
	defer connection.Close()

	name := connection.Names()[0]

	foo := &fooExport{}
	bar := &barExport{}
	connection.Export(foo, foopathstr, test1intfstr)
	connection.Export(foo, foopathstr, test2intfstr)
	connection.Export(bar, barpathstr, test2intfstr)
	connection.Export(intro, pathstr, introIntf)

	var response string
	var match bool
	path := strings.Split(pathstr, "/")
	for i := 0; i < len(path)-1; i++ {
		var subpath string
		if i == 0 {
			subpath = "/"
		} else {
			subpath = strings.Join(path[:i+1], "/")
		}

		object := connection.Object(name, ObjectPath(subpath))
		err = object.Call(introIntf+".Introspect", 0).Store(&response)
		if err != nil {
			t.Errorf("Unexpected error calling Introspect on %s: %s", subpath, err)
		}

		exp := fmt.Sprintf(respTmpl, path[i+1])
		match, err = regexp.MatchString(exp, response)
		if err != nil {
			t.Fatalf("Error calling MatchString: %s", err)
		}
		if !match {
			t.Errorf("Unexpected introspection response for %s: %s", subpath, response)
		}
	}

	// Test invalid subpath
	invalSubpath := "/org/guelfey/DBus/Test/Nonexistent"
	object := connection.Object(name, ObjectPath(invalSubpath))
	err = object.Call(introIntf+".Introspect", 0).Store(&response)
	if err != nil {
		t.Errorf("Unexpected error calling Introspect on %s: %s", invalSubpath, err)
	}
	match, err = regexp.MatchString(`^<node>\s*</node>$`, response)
	if err != nil {
		t.Fatalf("Error calling MatchString: %s", err)
	}
	if !match {
		t.Errorf("Unexpected introspection response for %s: %s", invalSubpath, response)
	}
}
