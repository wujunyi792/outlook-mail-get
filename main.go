package main

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		exitWithError(err)
	}
}
