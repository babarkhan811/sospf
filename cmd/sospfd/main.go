// Command sospfd is the S-OSPF router daemon. In Phase 2 the runtime transports
// (UDP overlay, raw IP) are not yet wired, so the daemon directs users to the
// emulator; it becomes a real daemon when the overlay transport lands.
package main

import "fmt"

func main() {
	fmt.Println("sospfd: the standalone daemon is not wired yet (phase 2 runs in the emulator).")
	fmt.Println("Run a topology with:  sospf-emu -topo topologies/triangle.json")
}
