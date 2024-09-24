package urlbuilder

import (
	"net/url"
	"strings"

	"github.com/a-h/templ"
)

// URLBuilder is a builder for constructing URLs
type URLBuilder struct {
	scheme   string
	host     string
	path     []val
	query    url.Values
	fragment string
}

type val struct {
	val          string
	shouldEscape bool
}

// New creates a new URLBuilder with the given scheme and host
func New() *URLBuilder {
	return &URLBuilder{
		query: make(url.Values),
	}
}

// Scheme creates a new URLBuilder with the given scheme
func Scheme(scheme string) *URLBuilder {
	return New().Scheme(scheme)
}

// Host creates a new URLBuilder with the given host
// The scheme will be protocol-relative
func Host(host string) *URLBuilder {
	return New().Host(host)
}

// Path creates a new URLBuilder with the given path segment
// This path will *not* be escaped
func Path(segment string) *URLBuilder {
	ub := New()
	ub.path = []val{{val: segment, shouldEscape: false}}
	return ub
}

// Scheme sets the scheme of the URL
func (ub *URLBuilder) Scheme(scheme string) *URLBuilder {
	ub.scheme = scheme
	return ub
}

// Host sets the host of the URL
func (ub *URLBuilder) Host(host string) *URLBuilder {
	ub.host = host
	return ub
}

// Path adds a path segment to the URL
func (ub *URLBuilder) Path(segment string) *URLBuilder {
	ub.path = append(ub.path, val{val: segment, shouldEscape: true})
	return ub
}

// Query adds a query parameter to the URL
func (ub *URLBuilder) Query(key string, value string) *URLBuilder {
	ub.query.Add(key, value)
	return ub
}

// Fragment sets the fragment (hash) part of the URL
func (ub *URLBuilder) Fragment(fragment string) *URLBuilder {
	ub.fragment = fragment
	return ub
}

// Build constructs the final URL as a SafeURL
func (ub *URLBuilder) Build() templ.SafeURL {
	var buf strings.Builder
	switch ub.scheme {
	case "tel", "mailto":
		buf.WriteString(ub.scheme)
		buf.WriteByte(':')
		buf.WriteString(ub.host)
		return templ.SafeURL(buf.String())
	default:
		if ub.scheme != "" {
			buf.WriteString(ub.scheme)
			buf.WriteByte(':')
		}
	}

	if ub.host != "" {
		buf.WriteString("//")
		buf.WriteString(ub.host)
	}

	for _, segment := range ub.path {

		if !strings.HasPrefix(segment.val, "/") {
			buf.WriteByte('/')
		}
		if segment.shouldEscape {
			buf.WriteString(url.PathEscape(segment.val))
		} else {
			buf.WriteString(segment.val)
		}
	}

	if len(ub.query) > 0 {
		buf.WriteByte('?')
		buf.WriteString(ub.query.Encode())
	}

	if ub.fragment != "" {
		buf.WriteByte('#')
		buf.WriteString(url.QueryEscape(ub.fragment))
	}

	return templ.SafeURL((buf.String()))
}
