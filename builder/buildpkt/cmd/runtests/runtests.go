package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func chkErr0(e error) {
	if e != nil {
		panic(e)
	}
}

func chkErr1[T interface{}](t T, e error) T {
	chkErr0(e)
	return t
}

type Sema struct {
	wg sync.WaitGroup
	ch chan struct{}
}

func (s *Sema) Do(f func()) {
	s.wg.Add(1)
	go func() {
		s.ch <- struct{}{}
		f()
		<-s.ch
		s.wg.Done()
	}()
}
func (s *Sema) Wait() {
	s.wg.Wait()
}
func NewSema(size int) *Sema {
	return &Sema{
		ch: make(chan struct{}, size),
	}
}

const maxAttempts int = 10

func runTest(testbin, outdir, testdir, test string, sema *Sema, attempt int) {
	sema.Do(func() {
		cmd := exec.Command(testbin, "-test.run", "^"+test+"$", "-test.timeout=60s")
		cmd.Env = append(os.Environ(), "GOMAXPROCS=1")
		cmd.Dir = testdir
		filename := filepath.Join(outdir, test+".run.txt")
		f := chkErr1(os.Create(filename))
		chkErr1(f.WriteString(
			fmt.Sprintf("%s %s %s %s\n\n", testbin, "-test.run", "^"+test+"$", "-test.timeout=60s"),
		))
		cmd.Stdout = f
		cmd.Stderr = f
		err := cmd.Run()
		f.Close()
		failName := filepath.Join(outdir, test+".fail.txt")
		passName := filepath.Join(outdir, test+".pass.txt")
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				fmt.Printf("FAIL %s (attempt: %d)\n", failName, attempt)
				chkErr0(os.Rename(filename, failName))
				if attempt < maxAttempts {
					runTest(testbin, outdir, testdir, test, sema, attempt+1)
				}
			} else {
				chkErr0(err)
			}
		} else {
			os.Remove(failName)
			chkErr0(os.Rename(filename, passName))
			fmt.Printf("PASS %s\n", passName)
		}
	})
}

func runTestPackage(outdir, testdir string, sema *Sema) {
	sema.Do(func() {
		fmt.Printf("running package %s\n", testdir)
		testFile := filepath.Join(outdir, "testbin")
		cmd := exec.Command("go", "test", "-tags", "dev", "-o", testFile, "-c")
		cmd.Env = append(os.Environ(), "GOMAXPROCS=1")
		cmd.Dir = testdir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		chkErr0(err)

		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			return
		}

		tests := bytes.NewBuffer(nil)
		cmd = exec.Command(testFile, "-test.list=.")
		cmd.Env = append(os.Environ(), "GOMAXPROCS=1")
		cmd.Dir = testdir
		cmd.Stdout = tests
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		chkErr0(err)

		for _, line := range strings.Split(tests.String(), "\n") {
			line := strings.TrimSpace(line)
			if line == "" {
				continue
			}
			runTest(testFile, outdir, testdir, line, sema, 1)
		}
	})
}

var threads int = runtime.NumCPU()

func main() {
	cwd := chkErr1(os.Getwd())
	outDir := filepath.Join(cwd, "testout")
	chkErr0(os.RemoveAll(outDir))
	chkErr0(os.Mkdir(outDir, 0755))
	paths := make(map[string]struct{})
	sema := NewSema(threads)

	// this directory requires a special tag to compile
	paths["lnd/channeldb/kvdb/etcd"] = struct{}{}

	chkErr0(filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if !strings.Contains(info.Name(), "_test.go") {
			return nil
		}
		testDir, _ := filepath.Split(path)
		relTestDir := chkErr1(filepath.Rel(cwd, testDir))
		if _, ok := paths[relTestDir]; ok {
			return nil
		}
		paths[relTestDir] = struct{}{}
		outDir := filepath.Join(outDir, relTestDir)
		chkErr0(os.MkdirAll(outDir, 0755))
		runTestPackage(outDir, testDir, sema)
		return nil
	}))
	sema.Wait()

	fmt.Println("---")
	fmt.Println()
	failed := false
	filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".fail.txt") {
			fmt.Printf("FAIL %s\n", path)
			failed = true
		}
		return nil
	})
	if failed {
		os.Exit(100)
	}
}
