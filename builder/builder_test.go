package builder

import (
	"fmt"
	"testing"
	// Correct import path for openresty name logic, if needed for direct testing.
	// However, for builder tests, we rely on its public interface.
	// "github.com/cubicdaiya/nginx-build/openresty"
)

func TestMakeBuilder(t *testing.T) {
	testCases := []struct {
		name      string
		component ComponentType
		version   string
		static    bool
		expected  Builder
	}{
		{
			name:      "Nginx main",
			component: ComponentNginx,
			version:   "1.25.0",
			static:    false,
			expected:  Builder{Component: ComponentNginx, Version: "1.25.0", Static: false, DownloadURLPrefix: NginxDownloadURLPrefix},
		},
		{
			name:      "PCRE static",
			component: ComponentPcre,
			version:   "10.42",
			static:    true,
			expected:  Builder{Component: ComponentPcre, Version: "10.42", Static: true, DownloadURLPrefix: fmt.Sprintf("%s/pcre2-%s", PcreDownloadURLPrefix, "10.42")},
		},
		{
			name:      "OpenSSL static",
			component: ComponentOpenSSL,
			version:   "3.0.0",
			static:    true,
			expected:  Builder{Component: ComponentOpenSSL, Version: "3.0.0", Static: true, DownloadURLPrefix: fmt.Sprintf("%s/openssl-%s", OpenSSLDownloadURLPrefix, "3.0.0")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := MakeBuilder(tc.component, tc.version, tc.static)
			if b.Component != tc.expected.Component {
				t.Errorf("Expected Component %s, got %s", tc.expected.Component, b.Component)
			}
			if b.Version != tc.expected.Version {
				t.Errorf("Expected Version %s, got %s", tc.expected.Version, b.Version)
			}
			if b.Static != tc.expected.Static {
				t.Errorf("Expected Static %t, got %t", tc.expected.Static, b.Static)
			}
			if b.DownloadURLPrefix != tc.expected.DownloadURLPrefix {
				t.Errorf("Expected DownloadURLPrefix '%s', got '%s'", tc.expected.DownloadURLPrefix, b.DownloadURLPrefix)
			}
		})
	}
}

func TestBuilder_Name(t *testing.T) {
	// Behavior of openresty.Name(version):
	// sum > 1972 -> "openresty"
	// sum <= 1972 -> "ngx_openresty"
	// "1.21.4.1" -> sum 3141 -> "openresty"
	// "1.11.2.1" -> sum 2121 -> "openresty"
	// "1.9.7.2"  -> sum 1972 -> "ngx_openresty"
	// "1.9.3.0"  -> sum 1930 -> "ngx_openresty"
	testCases := []struct {
		component ComponentType
		version   string
		expected  string
	}{
		{ComponentNginx, "", "nginx"},
		{ComponentPcre, "", "pcre2"},
		{ComponentOpenSSL, "", "openssl"},
		{ComponentLibreSSL, "", "libressl"},
		{ComponentZlib, "", "zlib"},
		{ComponentOpenResty, "1.21.4.1", "openresty"},
		{ComponentOpenResty, "1.11.2.1", "openresty"},
		{ComponentOpenResty, "1.9.7.2", "ngx_openresty"},
		{ComponentOpenResty, "1.9.3.0", "ngx_openresty"},
		{ComponentFreenginx, "", "freenginx"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.component)+"_"+tc.version, func(t *testing.T) {
			b := MakeBuilder(tc.component, tc.version, false)
			if name := b.Name(); name != tc.expected {
				t.Errorf("Component %s, Version %s: Expected Name '%s', got '%s'", tc.component, tc.version, tc.expected, name)
			}
		})
	}
}

func TestBuilder_Option(t *testing.T) {
	testCases := []struct {
		name      string
		component ComponentType
		version   string // Relevant for OpenResty
		expected  string
	}{
		{"Nginx", ComponentNginx, "", "--with-nginx"},
		{"PCRE", ComponentPcre, "", "--with-pcre"},
		{"OpenSSL", ComponentOpenSSL, "", "--with-openssl"},
		{"LibreSSL", ComponentLibreSSL, "", "--with-openssl"},
		{"Zlib", ComponentZlib, "", "--with-zlib"},
		{"OpenResty_Modern", ComponentOpenResty, "1.21.4.1", "--with-openresty"},
		{"OpenResty_Legacy", ComponentOpenResty, "1.9.7.2", "--with-ngx_openresty"},
		{"Freenginx", ComponentFreenginx, "", "--with-freenginx"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := MakeBuilder(tc.component, tc.version, false)
			if option := b.Option(); option != tc.expected {
				t.Errorf("Component %s, Version %s: Expected Option '%s', got '%s' (Name was '%s')", tc.component, tc.version, tc.expected, option, b.Name())
			}
		})
	}
}

func TestBuilder_DownloadURL(t *testing.T) {
	testCases := []struct {
		name      string
		component ComponentType
		version   string
		expected  string
	}{
		{"Nginx", ComponentNginx, "1.25.0", "https://nginx.org/download/nginx-1.25.0.tar.gz"},
		{"PCRE2", ComponentPcre, "10.42", "https://github.com/PCRE2Project/pcre2/releases/download/pcre2-10.42/pcre2-10.42.tar.gz"},
		{"OpenSSL", ComponentOpenSSL, "3.0.8", "https://github.com/openssl/openssl/releases/download/openssl-3.0.8/openssl-3.0.8.tar.gz"},
		{"LibreSSL", ComponentLibreSSL, "3.7.2", "https://ftp.openbsd.org/pub/OpenBSD/LibreSSL/libressl-3.7.2.tar.gz"},
		{"Zlib", ComponentZlib, "1.2.13", "https://zlib.net/zlib-1.2.13.tar.gz"},
		{"OpenResty_Modern", ComponentOpenResty, "1.21.4.1", "https://openresty.org/download/openresty-1.21.4.1.tar.gz"},
		{"OpenResty_Legacy", ComponentOpenResty, "1.9.7.2", "https://openresty.org/download/ngx_openresty-1.9.7.2.tar.gz"}, // Name part changes
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := MakeBuilder(tc.component, tc.version, false)
			// For OpenResty, DownloadURL uses b.Name() which is derived from openresty.Name(version)
			// The expected URL needs to match this.
			expectedURL := tc.expected
			if tc.component == ComponentOpenResty {
				// The DownloadURL for OpenResty is fmt.Sprintf("%s/%s-%s.tar.gz", OpenRestyDownloadURLPrefix, b.Name(), builder.Version)
				// So if b.Name() is "openresty" for 1.21.4.1, URL is ".../openresty-1.21.4.1.tar.gz"
				// If b.Name() is "ngx_openresty" for 1.9.7.2, URL is ".../ngx_openresty-1.9.7.2.tar.gz"
				// This is correctly handled by the test cases.
			}

			if url := b.DownloadURL(); url != expectedURL {
				t.Errorf("Component %s, Version %s: Expected URL '%s', got '%s' (Name was '%s')", tc.component, tc.version, expectedURL, url, b.Name())
			}
		})
	}
}

func TestBuilder_SourcePath(t *testing.T) {
	// builder.SourcePath() is fmt.Sprintf("%s-%s", builder.Name(), builder.Version)
	testCases := []struct {
		name      string
		component ComponentType
		version   string
		expected  string
	}{
		{"Nginx", ComponentNginx, "1.25.0", "nginx-1.25.0"},
		{"PCRE2", ComponentPcre, "10.42", "pcre2-10.42"},
		// openresty.Name("1.21.4.1") -> "openresty"
		// -> SourcePath: "openresty-1.21.4.1"
		{"OpenResty_Modern", ComponentOpenResty, "1.21.4.1", "openresty-1.21.4.1"},
		// openresty.Name("1.9.7.2") -> "ngx_openresty"
		// -> SourcePath: "ngx_openresty-1.9.7.2"
		{"OpenResty_Legacy", ComponentOpenResty, "1.9.7.2", "ngx_openresty-1.9.7.2"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := MakeBuilder(tc.component, tc.version, false)
			if path := b.SourcePath(); path != tc.expected {
				t.Errorf("Component %s, Version %s: Expected SourcePath '%s', got '%s' (Builder.Name was '%s')", tc.component, tc.version, tc.expected, path, b.Name())
			}
		})
	}
}
