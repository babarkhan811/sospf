// Command sospfctl is the S-OSPF control CLI. In Phase 2 it is a thin stub; the
// show/resolve-intent subcommands are wired to a running sospfd in later phases.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "neighbors", "lsdb", "sft", "spf", "agents", "trust", "resolve-intent":
		fmt.Printf("sospfctl %s: not available until the daemon RPC lands (phase 4+).\n", os.Args[1])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`sospfctl <command>
  neighbors        show neighbor adjacencies
  lsdb             show the link-state database
  sft              show the semantic forwarding table
  spf              show SPF results / routing table
  agents           show advertised agents
  trust            show trust scores
  resolve-intent   resolve an intent to a target agent`)
}
