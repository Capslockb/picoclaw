package main
import (
	"fmt"
	"os"
)
func main() {
	home, _ := os.UserHomeDir()
	fmt.Printf("Home: %s\n", home)
	fmt.Printf("PICOCLAW_HOME: %s\n", os.Getenv("PICOCLAW_HOME"))
}
