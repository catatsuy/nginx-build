package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/cubicdaiya/nginx-build/builder"
	"github.com/cubicdaiya/nginx-build/command"
	"github.com/cubicdaiya/nginx-build/configure"
	"github.com/cubicdaiya/nginx-build/module3rd"
	"github.com/cubicdaiya/nginx-build/util"
)

var (
	nginxBuildOptions Options
)

func init() {
	nginxBuildOptions = makeNginxBuildOptions()
	// Initialize StringFlag fields so flag.Var can append to them
	nginxBuildOptions.Patches = StringFlag{}
	nginxBuildOptions.AddModules = StringFlag{}
	nginxBuildOptions.AddDynamicModules = StringFlag{}
}

func main() {
	// Parse flags
	for k, v := range nginxBuildOptions.Bools {
		v.Enabled = flag.Bool(k, false, v.Desc)
		nginxBuildOptions.Bools[k] = v
	}
	// Simplified loop for nginxBuildOptions.Values
	for k, v := range nginxBuildOptions.Values {
		v.Value = flag.String(k, v.Default, v.Desc)
		nginxBuildOptions.Values[k] = v
	}
	for k, v := range nginxBuildOptions.Numbers {
		v.Value = flag.Int(k, v.Default, v.Desc)
		nginxBuildOptions.Numbers[k] = v
	}

	// Register multi-value flags directly using nginxBuildOptions fields
	flag.Var(&nginxBuildOptions.Patches, "patch", "patch path for applying to nginx (can be used multiple times)")
	flag.Var(&nginxBuildOptions.AddModules, "add-module", "add 3rd party module (can be used multiple times)")
	flag.Var(&nginxBuildOptions.AddDynamicModules, "add-dynamic-module", "add 3rd party dynamic module (can be used multiple times)")

	var configureOptions configure.Options

	argsBool := configure.MakeArgsBool()
	for k, v := range argsBool {
		v.Enabled = flag.Bool(k, false, v.Desc)
		argsBool[k] = v
	}

	flag.CommandLine.SetOutput(os.Stdout)
	// The output of original flag.Usage() is too long
	defaultUsage := flag.Usage
	flag.Usage = usage
	flag.Parse()

	jobs := nginxBuildOptions.Numbers["j"].Value

	verbose := nginxBuildOptions.Bools["verbose"].Enabled
	pcreStatic := nginxBuildOptions.Bools["pcre"].Enabled
	openSSLStatic := nginxBuildOptions.Bools["openssl"].Enabled
	libreSSLStatic := nginxBuildOptions.Bools["libressl"].Enabled
	zlibStatic := nginxBuildOptions.Bools["zlib"].Enabled
	clear := nginxBuildOptions.Bools["clear"].Enabled
	versionPrint := nginxBuildOptions.Bools["version"].Enabled
	versionsPrint := nginxBuildOptions.Bools["versions"].Enabled
	openResty := nginxBuildOptions.Bools["openresty"].Enabled
	freenginx := nginxBuildOptions.Bools["freenginx"].Enabled
	configureOnly := nginxBuildOptions.Bools["configureonly"].Enabled
	idempotent := nginxBuildOptions.Bools["idempotent"].Enabled
	helpAll := nginxBuildOptions.Bools["help-all"].Enabled

	version := nginxBuildOptions.Values["v"].Value
	nginxConfigurePath := nginxBuildOptions.Values["c"].Value
	modulesConfPath := nginxBuildOptions.Values["m"].Value
	workParentDir := nginxBuildOptions.Values["d"].Value
	pcreVersion := nginxBuildOptions.Values["pcreversion"].Value
	openSSLVersion := nginxBuildOptions.Values["opensslversion"].Value
	libreSSLVersion := nginxBuildOptions.Values["libresslversion"].Value
	zlibVersion := nginxBuildOptions.Values["zlibversion"].Value
	openRestyVersion := nginxBuildOptions.Values["openrestyversion"].Value
	freenginxVersion := nginxBuildOptions.Values["freenginxversion"].Value
	patchOption := nginxBuildOptions.Values["patch-opt"].Value

	// Multi-value flags (Patches, AddModules, AddDynamicModules) are now directly in nginxBuildOptions.
	// The blocks for converting multiflag* to strings and assigning back are removed.

	// For `patchPath`, use the first patch specified, if any.
	var singlePatchFile string
	if len(nginxBuildOptions.Patches) > 0 {
		singlePatchFile = nginxBuildOptions.Patches[0]
		if len(nginxBuildOptions.Patches) > 1 {
			log.Printf("[notice] Multiple -patch flags provided. Only the first one ('%s') will be used by the patching process.", singlePatchFile)
		}
	}

	// Populate configureOptions.Values for configure.Generate.
	// argsString is initially empty (from configure.MakeArgsString()).
	// We ensure it's initialized if we need to add module paths.
	currentConfigureValues := make(map[string]configure.OptionValue)
	if len(nginxBuildOptions.AddModules) > 0 {
		addModulesValue := strings.Join(nginxBuildOptions.AddModules, ",")
		currentConfigureValues["add-module"] = configure.OptionValue{Value: &addModulesValue}
	}
	if len(nginxBuildOptions.AddDynamicModules) > 0 {
		addDynamicModulesValue := strings.Join(nginxBuildOptions.AddDynamicModules, ",")
		currentConfigureValues["add-dynamic-module"] = configure.OptionValue{Value: &addDynamicModulesValue}
	}
	configureOptions.Values = currentConfigureValues // Assign our map to configureOptions
	configureOptions.Bools = argsBool                // argsBool is from configure.MakeArgsBool() which is empty
	// Note: patchPath is replaced by singlePatchFile in subsequent code.
	parsedArgs := flag.Args() // Get non-flag arguments

	if *helpAll {
		defaultUsage()
		return
	}

	if *versionPrint {
		printNginxBuildVersion()
		return
	}

	if *versionsPrint {
		printNginxVersions()
		return
	}

	printFirstMsg()

	// set verbose mode
	command.VerboseEnabled = *verbose

	var nginxBuilder builder.Builder
	if *openResty && *freenginx {
		log.Fatal("select one between '-openresty' and '-freenginx'.")
	}
	if *openSSLStatic && *libreSSLStatic {
		log.Fatal("select one between '-openssl' and '-libressl'.")
	}
	// Main component builders - static is false or not applicable in the same sense as libraries.
	// Assuming 'false' for the static parameter for these.
	if *openResty {
		nginxBuilder = builder.MakeBuilder(builder.ComponentOpenResty, *openRestyVersion, false) // String consts already match
	} else if *freenginx {
		nginxBuilder = builder.MakeBuilder(builder.ComponentFreenginx, *freenginxVersion, false) // String consts already match
	} else {
		nginxBuilder = builder.MakeBuilder(builder.ComponentNginx, *version, false) // String consts already match
	}
	// Library builders - pass the respective *Static flag
	pcreBuilder := builder.MakeBuilder(builder.ComponentPcre, *pcreVersion, *pcreStatic)                 // String consts already match
	openSSLBuilder := builder.MakeBuilder(builder.ComponentOpenSSL, *openSSLVersion, *openSSLStatic)     // String consts already match
	libreSSLBuilder := builder.MakeBuilder(builder.ComponentLibreSSL, *libreSSLVersion, *libreSSLStatic) // String consts already match
	zlibBuilder := builder.MakeBuilder(builder.ComponentZlib, *zlibVersion, *zlibStatic)                 // String consts already match

	if *idempotent {
		builders := []builder.Builder{
			nginxBuilder,
			pcreBuilder,
			openSSLBuilder,
			libreSSLBuilder,
			zlibBuilder,
		}

		isSame, err := builder.IsSameVersion(builders)
		if err != nil {
			log.Println("[notice]", err)
		}
		if isSame {
			log.Println("Installed nginx is same.")
			return
		}
	}

	// change default umask
	_ = syscall.Umask(0)

	versionCheck(*version)

	nginxConfigure, err := util.FileGetContents(*nginxConfigurePath)
	if err != nil {
		log.Fatal(err)
	}
	nginxConfigure = configure.Normalize(nginxConfigure)

	modules3rd, err := module3rd.Load(*modulesConfPath)
	if err != nil {
		log.Fatal(err)
	}

	if len(*workParentDir) == 0 {
		log.Fatal("set working directory with -d")
	}

	if !util.FileExists(*workParentDir) {
		err := os.Mkdir(*workParentDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create working directory(%s) does not exist.", *workParentDir)
		}
	}

	var workDir string
	if *openResty {
		workDir = *workParentDir + "/openresty/" + *openRestyVersion
	} else if *freenginx {
		workDir = *workParentDir + "/freenginx/" + *freenginxVersion
	} else {
		workDir = *workParentDir + "/nginx/" + *version
	}

	if *clear {
		err := util.ClearWorkDir(workDir)
		if err != nil {
			log.Fatal(err)
		}
	}

	if !util.FileExists(workDir) {
		err := os.MkdirAll(workDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create working directory(%s) does not exist.", workDir)
		}
	}

	rootDir, err := util.SaveCurrentDir()
	if err != nil {
		log.Fatalf("Failed to save current directory: %v", err)
	}
	err = os.Chdir(workDir)
	if err != nil {
		log.Fatalf("Failed to change directory to %s: %v", workDir, err)
	}

	// remove nginx source code applyed patch
	if singlePatchFile != "" && util.FileExists(nginxBuilder.SourcePath()) {
		err := os.RemoveAll(nginxBuilder.SourcePath())
		if err != nil {
			log.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	if *pcreStatic {
		wg.Add(1)
		go func() {
			builder.DownloadAndExtractComponent(&pcreBuilder)
			wg.Done()
		}()
	}

	if *openSSLStatic {
		wg.Add(1)
		go func() {
			builder.DownloadAndExtractComponent(&openSSLBuilder)
			wg.Done()
		}()
	}

	if *libreSSLStatic {
		wg.Add(1)
		go func() {
			builder.DownloadAndExtractComponent(&libreSSLBuilder)
			wg.Done()
		}()
	}

	if *zlibStatic {
		wg.Add(1)
		go func() {
			builder.DownloadAndExtractComponent(&zlibBuilder)
			wg.Done()
		}()
	}

	wg.Add(1)
	go func() {
		builder.DownloadAndExtractComponent(&nginxBuilder)
		wg.Done()
	}()

	if len(modules3rd) > 0 {
		wg.Add(len(modules3rd))
		for _, mod := range modules3rd { // Renamed m to mod to avoid conflict
			go func(m module3rd.Module3rd) { // Keep m for the goroutine's copy
				defer wg.Done()
				logFile := fmt.Sprintf("%s.log", m.Name) // Determine log file name for this module
				err := module3rd.DownloadAndExtractParallel(m)
				if err != nil {
					util.PrintFatalMsg(err, logFile) // util.PrintFatalMsg will log and exit
				}
			}(mod) // Pass mod (the loop variable)
		}
	}

	// wait until all downloading processes by goroutine finish
	wg.Wait()

	if len(modules3rd) > 0 {
		for _, m := range modules3rd {
			if err := module3rd.Provide(&m); err != nil {
				log.Fatal(err)
			}
		}
	}

	// cd workDir/nginx-${version}
	os.Chdir(nginxBuilder.SourcePath())

	var dependencies []*builder.Builder // Changed type here
	if *pcreStatic {
		dependencies = append(dependencies, &pcreBuilder) // Add pointer to builder
	}

	if *openSSLStatic {
		dependencies = append(dependencies, &openSSLBuilder) // Add pointer to builder
	}

	if *libreSSLStatic {
		dependencies = append(dependencies, &libreSSLBuilder) // Add pointer to builder
	}

	if *zlibStatic {
		dependencies = append(dependencies, &zlibBuilder) // Add pointer to builder
	}

	log.Printf("Generate configure script for %s.....", nginxBuilder.SourcePath())

	if *pcreStatic && pcreBuilder.IsIncludeWithOption(nginxConfigure) {
		log.Println(pcreBuilder.WarnMsgWithLibrary())
	}

	if *openSSLStatic && openSSLBuilder.IsIncludeWithOption(nginxConfigure) {
		log.Println(openSSLBuilder.WarnMsgWithLibrary())
	}

	if *libreSSLStatic && libreSSLBuilder.IsIncludeWithOption(nginxConfigure) {
		log.Println(libreSSLBuilder.WarnMsgWithLibrary())
	}

	if *zlibStatic && zlibBuilder.IsIncludeWithOption(nginxConfigure) {
		log.Println(zlibBuilder.WarnMsgWithLibrary())
	}

	// configure.Generate now returns a single string, as it was reverted to its pre-template, pre-error-return state
	// and then adapted. The adapted non-template version does not have internal error conditions.
	configureScript := configure.Generate(nginxConfigure, modules3rd, dependencies, configureOptions, rootDir, *openResty, *jobs, parsedArgs)

	err = os.WriteFile("./nginx-configure", []byte(configureScript), 0655)
	if err != nil {
		log.Fatalf("Failed to write configure script for %s: %v", nginxBuilder.SourcePath(), err)
	}

	if err := util.Patch(singlePatchFile, *patchOption, rootDir, false); err != nil {
		log.Fatalf("Failed to apply patch: %v", err)
	}

	// reverts source code with patch -R when the build was interrupted.
	if singlePatchFile != "" {
		sigChannel := make(chan os.Signal, 1)
		signal.Notify(sigChannel, os.Interrupt)
		go func() {
			<-sigChannel
			log.Println("Interrupt signal received. Attempting to revert patch...")
			if err := util.Patch(singlePatchFile, *patchOption, rootDir, true); err != nil {
				log.Printf("ERROR: Failed to revert patch %s: %v", singlePatchFile, err)
				// Not calling log.Fatal here as we are already in a signal handler,
				// and the main flow might be terminating.
			}
			os.Exit(1) // Exit after attempting to revert patch on interrupt
		}()
	}

	log.Printf("Configure %s.....", nginxBuilder.SourcePath())

	err = configure.Run()
	if err != nil {
		log.Printf("Failed to configure %s\n", nginxBuilder.SourcePath())
		if err := util.Patch(singlePatchFile, *patchOption, rootDir, true); err != nil {
			log.Printf("Additionally, failed to revert patch during configure error handling: %v", err)
		}
		util.PrintFatalMsg(err, "nginx-configure.log")
	}

	if *configureOnly {
		// Attempt to revert patch if configureOnly is set.
		if err := util.Patch(singlePatchFile, *patchOption, rootDir, true); err != nil {
			log.Printf("Warning: Failed to revert patch %s after configure only: %v", singlePatchFile, err)
		}
		printLastMsg(workDir, nginxBuilder.SourcePath(), *openResty, *configureOnly)
		return
	}

	log.Printf("Build %s.....", nginxBuilder.SourcePath())

	if *openSSLStatic {
		// Sometimes machine hardware name('uname -m') is different
		// from machine processor architecture name('uname -p') on Mac.
		// Specifically, `uname -p` is 'i386' and `uname -m` is 'x86_64'.
		// In this case, a build of OpenSSL fails.
		// So it needs to convince OpenSSL with KERNEL_BITS.
		if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
			os.Setenv("KERNEL_BITS", "64")
		}
	}

	err = builder.BuildNginx(*jobs)
	if err != nil {
		log.Printf("Failed to build %s\n", nginxBuilder.SourcePath())
		if err := util.Patch(singlePatchFile, *patchOption, rootDir, true); err != nil {
			log.Printf("Additionally, failed to revert patch during build error handling: %v", err)
		}
		util.PrintFatalMsg(err, "nginx-build.log")
	}

	// Successfully built, attempt to revert patch if it was applied.
	// This is to leave the source tree clean.
	if singlePatchFile != "" {
		if err := util.Patch(singlePatchFile, *patchOption, rootDir, true); err != nil {
			log.Printf("Warning: Failed to revert patch %s after successful build: %v", singlePatchFile, err)
		}
	}
	printLastMsg(workDir, nginxBuilder.SourcePath(), *openResty, *configureOnly)
}
