package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func die(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

func assertNil(x interface{}, format string, args ...interface{}) {
	if x != nil {
		die(format, args...)
	}
}

type exeF int

const (
	exeEcho       exeF = 1 << iota
	exeCanFail    exeF = 1 << iota
	exeNoRedirect exeF = 1 << iota
)

type execer []string

var noEnvExecer execer = []string{}

func exe(flags exeF, name string, arg ...string) (int, string, string) {
	return noEnvExecer.exe(flags, name, arg...)
}

func (e *execer) exe(flags exeF, name string, arg ...string) (int, string, string) {
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	if flags&exeEcho != 0 {
		fmt.Println(strings.Join(append([]string{name}, arg...), " "))
	}
	cmd := exec.Command(name, arg...)
	if len(*e) > 0 {
		cmd.Env = append([]string{}, *e...)
	}
	if flags&exeNoRedirect == 0 {
		cmd.Stdout = stdout
		cmd.Stderr = stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	ret := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && flags&exeCanFail != 0 {
			ret = ee.ExitCode()
		} else {
			die("exe(%s, %v) -> %v", name, arg, err)
		}
	}
	return ret, stdout.String(), stderr.String()
}

func needUpdate(inPath string, outPaths []string) bool {
	if inFileStat, err := os.Stat(inPath); err != nil {
		panic(err)
	} else {
		for _, p := range outPaths {
			if fileStat, err := os.Stat(p); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Printf("GENERATING   %s\n", p)
					return true
				} else {
					panic(err)
				}
			} else if inFileStat.ModTime().After(fileStat.ModTime()) {
				fmt.Printf("REGENERATING %s\n", p)
				return true
			}
		}
	}
	return false
}

func ensureDir(dir string) {
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			os.MkdirAll(dir, 0755)
		} else {
			panic(err)
		}
	}
}

func processProto(inPath string, e *execer) bool {
	inDir, file := filepath.Split(inPath)
	outDir := "./generated/" + inDir
	jsonfile := file + ".doc.json"
	jsonPath := filepath.Join(outDir, jsonfile)
	ensureDir(outDir)
	if needUpdate(inPath, []string{jsonPath}) {
		e.exe(exeNoRedirect, "protoc",
			"-I./lnd/lnrpc",
			"--pkt-json_opt=json,"+file+".doc.json",
			"--pkt-json_out="+outDir,
			inPath,
		)
		return true
	}
	return false
}

func genproto() {
	e := execer{"PATH=" + os.Getenv("PATH") + ":./builder/buildpkt/bin"}
	didUpdate := false
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == ".proto" {
			didUpdate = processProto(path, &e) || didUpdate
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	if didUpdate {
		helpPath := "generated/lnd/pkthelp/"
		helpFile := helpPath + "helpgen.go"
		fmt.Printf("GENERATING   %s\n", helpFile)
		ensureDir(helpPath)
		_, out, err := e.exe(0, "./builder/buildpkt/bin/mkhelp", "./generated/lnd/lnrpc")
		if len(err) > 0 {
			fmt.Printf("mkhelp -> %s\n", err)
		}
		ioutil.WriteFile(helpFile, []byte(out), 0755)
	}
	fmt.Println("Protobufs generated")
}

// build stuff

type config struct {
	buildargs []string
	bindir    string
}

func build(name string, pkg string, conf *config) {
	if runtime.GOOS == "windows" {
		name = name + ".exe"
	}
	fmt.Printf("Building %s\n", name)
	args := append([]string{"build", "-o", conf.bindir + "/" + name}, conf.buildargs...)
	args = append(args, pkg)
	exe(exeNoRedirect|exeEcho, "go", args...)
}

func chkdir() {
	info, err := os.Stat("./builder/buildpkt/bin/build")
	if err != nil || info.IsDir() {
		die("this script must be invoked from the project root")
	}
}

func buildStr() string {
	exe(0, "git", "update-index", "-q", "--refresh")
	_, id, _ := exe(0, "git", "describe", "--tags", "HEAD")
	id = strings.TrimSpace(id)
	if x, _, _ := exe(exeCanFail, "git", "diff", "--quiet"); x != 0 {
		if os.Getenv("PKT_FAIL_DIRTY") != "" {
			die("Build is dirty, aborting")
		}
		return id + "-dirty"
	}
	return id
}

func ldflags() string {
	return "-X github.com/pkt-cash/pktd/pktconfig/version.appBuild=" + buildStr()
}

func test() {
	fmt.Println("Running tests")
	exe(exeNoRedirect, "go", "test", "-count=1", "-cover", "-parallel=1", "./...", "-tags=dev")
}

var regex = regexp.MustCompile("[A-Z0-9_]+=.*")

func main() {
	chkdir()
	genproto()
	conf := config{}
	conf.bindir = "./bin"
	conf.buildargs = append(conf.buildargs, "-trimpath")
	conf.buildargs = append(conf.buildargs, "-ldflags="+ldflags())

	assertNil(os.MkdirAll(conf.bindir, 0755), "mkdir bin")

	for _, a := range os.Args {
		if !regex.MatchString(a) {
			continue
		}
		i := strings.IndexRune(a, '=')
		fmt.Printf("env %s=%s\n", a[0:i], a[i+1:])
		os.Setenv(a[0:i], a[i+1:])
	}

	build("pktd", ".", &conf)
	build("pktwallet", "./pktwallet", &conf)
	build("pktctl", "./cmd/pktctl", &conf)
	build("checksig", "./cmd/checksig", &conf)
	build("pld", "./lnd/cmd/lnd", &conf)
	//	no need to compile and build the old version of pldctl
	//		build("pldctl", "./lnd/cmd/lncli", &conf)
	build("pldctl", "./lnd/cmd/lndcli", &conf)
	if strings.Contains(strings.Join(os.Args, "|"), "--test") {
		test()
	} else {
		fmt.Println("Pass the --test flag if you want to run the tests as well")
	}
	fmt.Println("Everything looks good, type `./bin/pktwallet --create` to make a wallet")
}
