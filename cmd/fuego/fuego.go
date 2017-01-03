///usr/bin/env true; exec /usr/bin/env go run "$0" "$@"

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/tzvetkoff-go/fuego"
	"golang.org/x/tools/imports"

	"github.com/fsnotify/fsnotify"
)

type batcher struct {
	watcher *fsnotify.Watcher
	done    chan bool
	events  chan string
}

func newBatcher(watcher *fsnotify.Watcher) *batcher {
	return &batcher{
		watcher: watcher,
		done:    make(chan bool),
		events:  make(chan string),
	}
}

func (b *batcher) add(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			b.watcher.Add(path)
		}

		return nil
	})
}

func (b *batcher) run() {
	ticker := time.Tick(100 * time.Millisecond)
	changes := map[string]bool{}

out:
	for {
		select {
		case event := <-b.watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				changes[event.Name] = true
			} else if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil {
					if info.IsDir() {
						b.add(event.Name)
						fmt.Println()
					}
				}
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				b.watcher.Remove(event.Name)
			}
		case <-ticker:
			for filename := range changes {
				b.events <- filename
			}
			changes = map[string]bool{}
		case <-b.done:
			break out
		}
	}
	close(b.done)
}

func (b *batcher) close() {
	b.done <- true
	b.watcher.Close()
}

func walk(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".ego" {
			compileAndWrite(path, path+".go")
		}

		return nil
	})
}

func compile(path string) ([]byte, error) {
	source, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	compiled, err := fuego.ParseBytes(source)
	if err != nil {
		return nil, err
	}

	processed, err := imports.Process(path+".go", compiled, &imports.Options{
		Fragment:   true,
		AllErrors:  false,
		Comments:   true,
		TabIndent:  true,
		TabWidth:   4,
		FormatOnly: false,
	})
	if err != nil {
		return nil, err
	}

	return processed, nil
}

func compileAndWrite(path string, out string) {
	fmt.Println("compiling: " + path + " -> " + path + ".go")
	t1 := time.Now()
	defer func() {
		t2 := time.Now()
		fmt.Printf("     done: %s\n\n", t2.Sub(t1))
	}()

	compiled, err := compile(path)
	if err != nil {
		fmt.Printf("    error: %s\n", err)
		return
	}

	err = ioutil.WriteFile(out, compiled, 0644)
	if err != nil {
		fmt.Printf("    error: %s\n", err)
		return
	}
}

func main() {
	compileCommandOut := ""
	compileCommand := &cobra.Command{
		Use:   "compile PATH",
		Short: "Compile a single file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			compiled, err := compile(args[0])
			if err != nil {
				return err
			}

			if compileCommandOut != "" {
				if err := ioutil.WriteFile(compileCommandOut, compiled, 0644); err != nil {
					return err
				}
			} else {
				fmt.Println(string(compiled))
			}

			return nil
		},
	}
	compileCommand.Flags().StringVarP(&compileCommandOut, "out", "o", compileCommandOut,
		"Write output to file instead of STDOUT")

	watchCommand := &cobra.Command{
		Use:   "watch DIR",
		Short: "Watch a directory for changes and compile files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return err
			}

			walk(args[0])

			batcher := newBatcher(watcher)
			defer batcher.close()

			batcher.add(args[0])
			fmt.Println(" watching: " + args[0] + "\n")

			go batcher.run()
		out:
			for {
				select {
				case path, ok := <-batcher.events:
					if !ok {
						break out
					}

					if filepath.Ext(path) == ".ego" {
						compileAndWrite(path, path+".go")
					}
				}
			}

			return nil
		},
	}

	rootCommand := &cobra.Command{
		Use:          "fuego",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}
	rootCommand.SetHelpCommand(&cobra.Command{})
	rootCommand.AddCommand(compileCommand, watchCommand)

	rootCommand.Execute()
}
