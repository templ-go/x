package urlbuilder

import (
	"testing"

	"github.com/a-h/templ"
)

func BenchmarkURLBuilder(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		New().
			Scheme("https").
			Host("example.com").
			Path("a").
			Path("b").
			Path("c").
			Query("key1", "value1").
			Query("key2", "value2").
			Query("key with space", "value with slash").
			Query("key/with/slash", "value/with/slash").
			Path("a/b").
			Query("key between paths", "value between paths").
			Path("c d").
			Fragment("fragment").Build()
	}
}

func TestBasicURL(t *testing.T) {
	t.Parallel()

	got := Scheme("https").
		Host("example.com").
		Build()

	expected := templ.URL("https://example.com")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestURLWithPaths(t *testing.T) {
	t.Parallel()

	c := "c"
	got := Scheme("https").
		Host("example.com").
		Path("a").
		Path("b").
		Path(c).
		Query("key", "value").Build()

	expected := templ.URL("https://example.com/a/b/c?key=value")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestURLWithMultipleQueries(t *testing.T) {
	t.Parallel()

	got := Scheme("https").
		Host("example.com").
		Path("path").
		Query("key1", "value1").
		Query("key2", "value2").
		Build()

	expected := templ.URL("https://example.com/path?key1=value1&key2=value2")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestURLWithNoPaths(t *testing.T) {
	t.Parallel()

	got := Scheme("https").
		Host("example.com").
		Query("search", "golang").
		Build()

	expected := templ.URL("https://example.com?search=golang")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestURLEscapingPath(t *testing.T) {
	t.Parallel()

	got := Scheme("https").
		Host("example.com").
		Path("a/b").
		Path("c d").
		Build()

	expected := templ.URL("https://example.com/a%2Fb/c%20d")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestURLEscapingQuery(t *testing.T) {
	t.Parallel()

	got := Scheme("https").
		Host("example.com").
		Query("key with space", "value with space").
		Query("key/with/slash", "value/with/slash").
		Build()

	expected := templ.URL("https://example.com?key+with+space=value+with+space&key%2Fwith%2Fslash=value%2Fwith%2Fslash")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestPath(t *testing.T) {
	t.Parallel()

	got := Path("chat").
		Path("response").
		Query("input", "hello!").
		Build()

	expected := templ.URL("/chat/response?input=hello%21")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestProtocolRelative(t *testing.T) {
	t.Parallel()

	got := Host("example.com").Build()

	expected := templ.URL("//example.com")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestSlash(t *testing.T) {
	t.Parallel()

	got := Path("/").Build()

	expected := templ.URL("/")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestSlashIndex(t *testing.T) {
	t.Parallel()

	got := Path("/index").Build()

	expected := templ.URL("/index")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestHTTP(t *testing.T) {
	t.Parallel()

	got := Scheme("http").Host("example.com").Build()

	expected := templ.URL("http://example.com")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestHTTPS(t *testing.T) {
	t.Parallel()

	got := Scheme("https").Host("example.com").Build()

	expected := templ.URL("https://example.com")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestMailTo(t *testing.T) {
	t.Parallel()

	got := Scheme("mailto").Host("test@example.com").Build()

	expected := templ.URL("mailto:test@example.com")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestTel(t *testing.T) {
	t.Parallel()

	got := Scheme("tel").Host("+1234567890").Build()

	expected := templ.URL("tel:+1234567890")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestFtp(t *testing.T) {
	t.Parallel()

	got := Scheme("ftp").Host("example.com").Build()

	expected := templ.URL("ftp://example.com")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}

func TestFtps(t *testing.T) {
	t.Parallel()

	got := Scheme("ftps").Host("example.com").Build()

	expected := templ.URL("ftps://example.com")

	if got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
}
