package main

func main() {
	wg.Add(4)

	// db connection
	dbClient, err := runDBConnection()
	checkError("Main: runDBConnection", err)
	defer dbClient.Close()

	// http connection with browser
	go runDynamicServer()

	// web socket server
	go websocketServer()

	//-----TCP-Config
	go runConfigServer(configConnType, configHost, configPort)
	//-----TCP
	go runTCPServer()

	wg.Wait()
}
