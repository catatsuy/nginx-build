package configure

import (
	"fmt"
	"strings"

	"github.com/cubicdaiya/nginx-build/builder"
	"github.com/cubicdaiya/nginx-build/module3rd"
)

// normalizeArg ensures arguments with spaces are properly quoted for shell.
// For key=value, it quotes only the value part if it contains spaces.
func normalizeArg(arg string) string {
	if strings.Contains(arg, " ") {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			// Quote value part if it contains space
			if strings.Contains(parts[1], " ") {
				return fmt.Sprintf("%s='%s'", parts[0], parts[1])
			}
			// No space in value, return as is
			return arg
		}
		// Not key=value, quote the whole argument if it contains space
		return fmt.Sprintf("'%s'", arg)
	}
	return arg // No spaces, return as is
}


func Generate(configure string, modules3rd []module3rd.Module3rd, dependencies []*builder.Builder, options Options, rootDir string, openResty bool, jobs int, flagArgs []string) string {
	openSSLStatic := false
	var argsList []string

	if openResty {
		argsList = append(argsList, fmt.Sprintf("-j%d", jobs))
	}

	for _, d := range dependencies {
		argsList = append(argsList, fmt.Sprintf("%s=../%s-%s", d.Option(), d.Name(), d.Version))
		if d.Component == builder.ComponentOpenSSL || d.Component == builder.ComponentLibreSSL {
			openSSLStatic = true
		}
	}

	// Add --with-http_ssl_module if OpenSSL/LibreSSL is static.
	// The check !strings.Contains(configure, ...) is applied to the base 'configure' script.
	if openSSLStatic && !strings.Contains(configure, "--with-http_ssl_module") {
		argsList = append(argsList, "--with-http_ssl_module")
	}

	// Process 3rd party modules from modules.conf
	formattedModules3rd := generateForModule3rd(modules3rd)
	if strings.TrimSpace(formattedModules3rd) != "" {
		for _, line := range strings.Split(strings.TrimSpace(formattedModules3rd), "\n") {
			argsList = append(argsList, strings.TrimSuffix(strings.TrimSpace(line), "\\"))
		}
	}

	// Process options from command line (via configureOptions)
	for _, option := range options.Values {
		if option.Value != nil && *option.Value != "" {
			val := *option.Value
			if option.Name == "add-module" {
				normalizedPaths := normalizeAddModulePaths(val, rootDir, false)
				for _, p := range strings.Split(strings.TrimSpace(normalizedPaths), "\n") {
					argsList = append(argsList, strings.TrimSuffix(strings.TrimSpace(p), "\\"))
				}
			} else if option.Name == "add-dynamic-module" {
				normalizedPaths := normalizeAddModulePaths(val, rootDir, true)
				for _, p := range strings.Split(strings.TrimSpace(normalizedPaths), "\n") {
					argsList = append(argsList, strings.TrimSuffix(strings.TrimSpace(p), "\\"))
				}
			} else {
				argsList = append(argsList, normalizeArg(fmt.Sprintf("%s=%s", option.Name, val)))
			}
		}
	}

	for _, option := range options.Bools {
		if option.Enabled != nil && *option.Enabled {
			argsList = append(argsList, option.Name)
		}
	}

	// Process passthrough flagArgs
	for _, arg := range flagArgs {
		argsList = append(argsList, normalizeArg(arg))
	}

	// Construct the final script
	var finalScriptBuilder strings.Builder
	if len(configure) == 0 {
		finalScriptBuilder.WriteString("#!/bin/sh\n\n./configure")
	} else {
		// If base configure script is provided, normalize its ending.
		// The external configure.Normalize function adds a trailing space.
		// Here, we ensure it's ready to have args appended.
		finalScriptBuilder.WriteString(strings.TrimSpace(configure))
	}

	if len(argsList) > 0 {
		finalScriptBuilder.WriteString(" \\\n    ") // Start args on new line
		finalScriptBuilder.WriteString(strings.Join(argsList, " \\\n    "))
	}
	finalScriptBuilder.WriteString("\n") // Ensure final newline

	return finalScriptBuilder.String()
}

func generateForModule3rd(modules3rd []module3rd.Module3rd) string {
	result := ""
	for _, m := range modules3rd {
		opt := "--add-module"
		if m.Dynamic {
			opt = "--add-dynamic-module"
		}
		if m.Form == "local" {
			result += fmt.Sprintf("%s=%s \\\n", opt, m.Url)
		} else {
			result += fmt.Sprintf("%s=../%s \\\n", opt, m.Name)
		}
	}
	return result
}
