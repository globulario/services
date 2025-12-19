package main

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error
	}
}
