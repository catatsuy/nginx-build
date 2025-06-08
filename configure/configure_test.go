package configure

import (
	"strings"
	"testing"

	"github.com/cubicdaiya/nginx-build/builder"
	"github.com/cubicdaiya/nginx-build/module3rd"
)

// Helper to trim whitespace and backslashes for easier comparison of configure lines
func cleanScriptLine(line string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "\\"))
}

// Helper to clean the whole script for comparison
func cleanScript(script string) []string {
	var cleanedLines []string
	for _, line := range strings.Split(script, "\n") {
		trimmedLine := cleanScriptLine(line)
		if trimmedLine != "" { // Only add non-empty lines
			cleanedLines = append(cleanedLines, trimmedLine)
		}
	}
	return cleanedLines
}

func TestGenerate(t *testing.T) {
	// Mocked builders for dependencies
	// MakeBuilder now takes (ComponentType, version, static bool)
	pcreBuilder := builder.MakeBuilder(builder.ComponentPcre, "10.42", true)
	sslBuilder := builder.MakeBuilder(builder.ComponentOpenSSL, "3.0.8", true)

	testCases := []struct {
		name                string
		baseScript          string
		modules3rd          []module3rd.Module3rd
		dependencies        []*builder.Builder // Changed from builder.StaticLibrary
		options             Options            // configure.Options
		rootDir             string
		openResty           bool
		jobs                int
		flagArgs            []string
		expectedLines       []string
		unexpectedLines     []string
	}{
		{
			name:       "Minimal_NoBase_NoOpts",
			baseScript: "",
			expectedLines: []string{
				"#!/bin/sh",
				"./configure",
			},
		},
		{
			name:       "Minimal_WithBase_NoOpts",
			baseScript: "#!/bin/bash\n# My custom configure\n./configure --custom-base",
			expectedLines: []string{
				"#!/bin/bash",
				"# My custom configure",
				"./configure --custom-base",
			},
		},
		{
			name: "WithStaticDeps_AutoSSLModule",
			dependencies: []*builder.Builder{&pcreBuilder, &sslBuilder}, // Pass pointers
			expectedLines: []string{
				"./configure",
				"--with-pcre=../pcre2-10.42",       // Name() for pcre is "pcre2"
				"--with-openssl=../openssl-3.0.8",
				"--with-http_ssl_module",          // Auto-added due to openssl static
			},
		},
		{
			name: "WithStaticDeps_SSLModuleInBase",
			baseScript: "./configure --with-http_ssl_module",
			dependencies: []*builder.Builder{&sslBuilder}, // Pass pointers
			expectedLines: []string{
				"./configure --with-http_ssl_module",
				"--with-openssl=../openssl-3.0.8",
			},
			// Ensure --with-http_ssl_module is not duplicated by template logic if already in base
			// Current Generate logic adds it if openSSLStatic and not in base. This test is valid.
		},
		{
			name:      "OpenRestyWithJobs",
			openResty: true,
			jobs:      4,
			expectedLines: []string{
				"./configure",
				"-j4",
			},
		},
		{
			name: "With3rdPartyModules",
			modules3rd: []module3rd.Module3rd{
				{Name: "ngx_devel_kit", Form: "git", Url: "https://github.com/simpl/ngx_devel_kit.git"},
				{Name: "echo-nginx-module", Form: "local", Url: "/path/to/echo"},
				{Name: "ngx_cool_mod", Dynamic: true, Form: "git", Url: "someurl"},
			},
			expectedLines: []string{
				"./configure",
				"--add-module=../ngx_devel_kit",
				"--add-module=/path/to/echo",
				"--add-dynamic-module=../ngx_cool_mod",
			},
		},
		{
			name: "WithOptionsValues_AddModules",
			options: Options{
				Values: map[string]OptionValue{
					"add-module": {Name: "add-module", Value: strPtr("path/modA,path/modB")},
					"add-dynamic-module": {Name: "add-dynamic-module", Value: strPtr("path/dynModC")},
					"--prefix": {Name: "--prefix", Value: strPtr("/opt/nginx test")},
				},
			},
			rootDir: "/build",
			expectedLines: []string{
				"./configure",
				"--add-module=/build/path/modA",
				"--add-module=/build/path/modB",
				"--add-dynamic-module=/build/path/dynModC",
				"--prefix='/opt/nginx test'",
			},
		},
		{
			name: "WithOptionsBools",
			options: Options{
				Bools: map[string]OptionBool{
					"--with-debug":         {Name: "--with-debug", Enabled: boolPtr(true)},
					"--without-http_gzip_module": {Name: "--without-http_gzip_module", Enabled: boolPtr(false)},
				},
			},
			expectedLines: []string{
				"./configure",
				"--with-debug",
			},
			unexpectedLines: []string{"--without-http_gzip_module"},
		},
		{
			name:     "WithPassthroughArgs",
			flagArgs: []string{"--with-http_realip_module", "--user=nginx", "--group=nginx group"},
			expectedLines: []string{
				"./configure",
				"--with-http_realip_module",
				"--user=nginx",
				"--group='nginx group'", // Corrected expectation based on normalizeArg
			},
		},
		{
			name: "ComplexCase",
			baseScript: "./configure --prefix=/srv --my-custom-base-flag",
			dependencies: []*builder.Builder{&pcreBuilder}, // Pass pointers
			modules3rd: []module3rd.Module3rd{ {Name: "test_mod", Form:"local", Url:"/modules/test_mod"}},
			options: Options{
				Values: map[string]OptionValue{"add-module": {Name:"add-module", Value: strPtr("rel/mod1")}},
				Bools: map[string]OptionBool{"--with-stream": {Name:"--with-stream", Enabled:boolPtr(true)}},
			},
			rootDir: "/abs/root",
			openResty: true,
			jobs: 2,
			flagArgs: []string{"--with-ipv6"},
			expectedLines: []string{
				"./configure --prefix=/srv --my-custom-base-flag",
				"-j2",
				"--with-pcre=../pcre2-10.42",
				"--add-module=/modules/test_mod",
				"--add-module=/abs/root/rel/mod1",
				"--with-stream",
				"--with-ipv6",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate now takes 8 arguments and returns a single string
			generatedScript := Generate(tc.baseScript, tc.modules3rd, tc.dependencies, tc.options, tc.rootDir, tc.openResty, tc.jobs, tc.flagArgs)
			// No error to check from Generate itself

			cleanedGeneratedLines := cleanScript(generatedScript)

			expectedMap := make(map[string]bool)
			for _, line := range tc.expectedLines {
				expectedMap[cleanScriptLine(line)] = true
			}

			generatedMap := make(map[string]bool)
			for _, line := range cleanedGeneratedLines {
				generatedMap[line] = true
			}

			for _, expected := range tc.expectedLines {
				cleanedExpected := cleanScriptLine(expected)
				if !generatedMap[cleanedExpected] {
					t.Errorf("Expected line missing:\nEXPECTED: '%s'\n\nGENERATED SCRIPT:\n%s\n-------", cleanedExpected, strings.Join(cleanedGeneratedLines, "\n"))
				}
			}

			if tc.unexpectedLines != nil {
				for _, unexpected := range tc.unexpectedLines {
					cleanedUnexpected := cleanScriptLine(unexpected)
					if generatedMap[cleanedUnexpected] {
						t.Errorf("Unexpected line found:\nUNEXPECTED: '%s'\n\nGENERATED SCRIPT:\n%s\n-------", cleanedUnexpected, strings.Join(cleanedGeneratedLines, "\n"))
					}
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
