// Command opscompile runs the cluster_operator Operational Knowledge Compiler.
//
// Usage:
//
//	go run ./ai_memory/domains/cluster_operator/opsknowledge/cmd/opscompile \
//	  -corpus /abs/path/docs/operational-knowledge \
//	  -out    /abs/path/golang/ai_memory/domains/cluster_operator/generated
//
// It is deterministic: same corpus → identical generated files. The generated
// files are committed; this command regenerates them when the corpus changes.
package main

import (
	"flag"
	"fmt"
	"os"

	opsknowledge "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/opsknowledge"
)

func main() {
	corpus := flag.String("corpus", "", "absolute path to docs/operational-knowledge")
	out := flag.String("out", "", "absolute path to the generated/ output directory")
	flag.Parse()
	if *corpus == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "opscompile: -corpus and -out are required")
		os.Exit(2)
	}
	if err := opsknowledge.GenerateToDir(*corpus, *out); err != nil {
		fmt.Fprintln(os.Stderr, "opscompile:", err)
		os.Exit(1)
	}
	fmt.Println("opscompile: wrote generated bundle to", *out)
}
