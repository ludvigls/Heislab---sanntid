package main

import (
	"flag"
	"fmt"
	"strconv"
	"time"

	"./fsm"
	"./io"
	orderdelegator "./orderdelegator"

	"./network/bcast"
	"./network/peers"
)

// CountMsg is a struct sending an alive message with id
type CountMsg struct {
	Message string
	ID      int // from
	Iter    int
}

// counter, func for testing network
// func counter(countCh chan<- int, startFrom int) {
// 	count := startFrom
// 	for {
// 		count++
// 		countCh <- count
// 		time.Sleep(1 * time.Second)
// 	}
// }

// isMaster returns true if the ID is the smallest on the network (in the PeerList)
func isMaster(PeerList []string, ID int, masterWaitDone bool) bool {
	if ID == -1 || !masterWaitDone {
		return false // Unitialized node cannot be master
	}
	for i := 0; i < len(PeerList); i++ {
		peerID, _ := strconv.Atoi(PeerList[i])
		if peerID < ID {
			return false
		}
	}
	return true
}

// initializeID initializes the ID to the id to highest_id+1
func initializeID(PeerList []string) int {
	fmt.Println("initializing id")
	fmt.Println(PeerList)
	highestID := -1
	peerID := -1
	for i := 0; i < len(PeerList); i++ {
		peerID, _ = strconv.Atoi(PeerList[i])
		if peerID > highestID {
			highestID = peerID
		}
	}
	return highestID + 1
}

// getMostRecentMsg, listens for messages for a while, gets the most recent message
func getMostRecentMsg(peerUpdateCh chan peers.PeerUpdate, PeerList []string) []string {
	//TODO : this function was made to prevent some bug
	//Atm it does nothing, but things still work...
	timeOut := false
	timer := time.NewTimer(200 * time.Millisecond) //emptys the message stack for 100ms
	for !timeOut {
		select {
		case <-timer.C:
			//fmt.Println("TIME OUT!!")
			timeOut = true
		case a := <-peerUpdateCh:
			//fmt.Println("THIS FUNCTION HAS A PURPOSE :0 !, UPDATING PEERLIST!!") // TODO CODE IS NEVER HERE!!!
			PeerList = a.Peers
		}
	}
	return PeerList
}

func main() { // `go run network_node.go -id=our_id` -liftPort=15657
	var idStr string
	//var count_glob int
	var PeerList []string
	//var hasBeenMaster bool
	initialized := false
	globState := make(map[string]fsm.State)
	numFloors := 4
	liftPort := "15657"

	flag.StringVar(&idStr, "id", "", "id of this peer")
	flag.StringVar(&liftPort, "port", "", "port to the lift connected")
	flag.Parse()

	io.Init("localhost:"+liftPort, numFloors)

	if idStr == "" {
		idStr = "-1"
	}

	idInt, _ := strconv.Atoi(idStr)

	// We make a channel for receiving updates on the id's of the peers that are alive on network
	peerUpdateCh := make(chan peers.PeerUpdate)
	peerTxEnable := make(chan bool) // We can disable/enable the transmitter after it has been started (This could be used to signal that we are somehow "unavailable".)

	// We make channels for sending and receiving our custom data types
	// countTx := make(chan CountMsg)
	// countRx := make(chan CountMsg)

	localStateTx := make(chan fsm.State)
	localStateRx := make(chan fsm.State)

	globStateTx := make(chan map[string]fsm.State)
	globStateRx := make(chan map[string]fsm.State)

	unassignedOrderTx := make(chan fsm.Order)
	unassignedOrderRx := make(chan fsm.Order)

	assignedOrderTx := make(chan fsm.Order)
	assignedOrderRx := make(chan fsm.Order)

	//countCh := make(chan int)
	//idCh := make(chan int)

	// GO ROUTINES EVERYONE WILL RUN v
	drv_buttons := make(chan io.ButtonEvent)
	drv_floors := make(chan int)
	fsm_n_orderCh := make(chan fsm.Order, 1000)
	n_od_orderCh := make(chan fsm.Order, 1000)
	od_n_orderCh := make(chan fsm.Order, 1000)
	n_fsm_orderCh := make(chan fsm.Order, 1000)
	n_od_globstateCh := make(chan map[string]fsm.State, 1000)
	fsm_n_stateCh := make(chan fsm.State, 1000)

	//Every node initialized as pure recievers
	go peers.Receiver(15647, peerUpdateCh)
	//go bcast.Receiver(16569, countRx)
	go bcast.Receiver(16570, localStateRx)
	go bcast.Receiver(16571, globStateRx)
	go bcast.Receiver(16572, unassignedOrderRx)
	go bcast.Receiver(16573, assignedOrderRx)

	if idInt != -1 { //Nodes with IDs are allowed to transmit
		//fmt.Println("Starting transmitting from ID: ", id)
		// go peers.Transmitter(15647, idStr, peerTxEnable)
		// //go bcast.Transmitter(16569, countTx)
		// go bcast.Transmitter(16570, localStateTx)
		// go bcast.Transmitter(16571, globStateTx)
		// go bcast.Transmitter(16572, unassignedOrderTx)
		// go bcast.Transmitter(16573, assignedOrderTx)
		go peers.Transmitter(15647, idStr, peerTxEnable)
		go fsm.Fsm(drv_buttons, drv_floors, numFloors, fsm_n_orderCh, n_fsm_orderCh, fsm_n_stateCh, idInt)
		go orderdelegator.OrderDelegator(n_od_orderCh, od_n_orderCh, n_od_globstateCh, numFloors)
	}

	go io.Io(drv_buttons, drv_floors)
	//go orderdelegator.OrderDelegator(n_od_orderCh, od_n_orderCh, n_od_globstateCh, numFloors)

	//Everyone sends out its count msg
	// go func(idCh chan int) {
	// 	CountMsg := CountMsg{"I'm sending the global state of all lifts", idInt, 0}
	// 	for {
	// 		select {
	// 		case a := <-idCh: //Needed when node is initialized without id
	// 			CountMsg.ID = a
	// 		default:
	// 			CountMsg.Iter = count_glob //Everyone sends the global state in its alive message
	// 			countTx <- CountMsg
	// 			time.Sleep(100 * time.Millisecond)
	// 		}
	// 	}
	// }(idCh)
	masterWait := time.NewTimer(200 * time.Millisecond)
	masterWaitDone := false
	for {

		select {
		case <-masterWait.C:
			masterWaitDone = true
		case p := <-peerUpdateCh:
			fmt.Printf("Peer update:\n")
			fmt.Printf("  Peers:    %q\n", p.Peers)
			fmt.Printf("  New:      %q\n", p.New)
			fmt.Printf("  Lost:     %q\n", p.Lost)

			PeerList = p.Peers
			if !initialized {
				PeerList = getMostRecentMsg(peerUpdateCh, PeerList) // tømme postkassa peerUpdateCh
				fmt.Printf("PeerList:    %q\n", PeerList)

				//go peers.Transmitter(15647, idStr, peerTxEnable)
				//go bcast.Transmitter(16569, countTx)

				fmt.Println("starting transmision after init")
				go bcast.Transmitter(16570, localStateTx)
				go bcast.Transmitter(16571, globStateTx)
				go bcast.Transmitter(16572, unassignedOrderTx)
				go bcast.Transmitter(16573, assignedOrderTx)

				initialized = true
			}

			/*
				if idInt == -1 {
					PeerList = getMostRecentMsg(peerUpdateCh, PeerList)
					idInt = initializeID(PeerList)

					//Initialize transmit features for node
					go peers.Transmitter(15647, strconv.Itoa(idInt), peerTxEnable)
					//go bcast.Transmitter(16569, countTx)
					go bcast.Transmitter(16570, localStateTx)

					fmt.Println("MY ID IS: ", idInt)
					go fsm.Fsm(drv_buttons, drv_floors, numFloors, fsm_n_orderCh, n_fsm_orderCh, fsm_n_stateCh, idInt)
					go orderdelegator.OrderDelegator(n_od_orderCh, od_n_orderCh, n_od_globstateCh, numFloors)

					idCh <- idInt
				}
			*/

			//Network is lost, all work as individual lifts
			if len(PeerList) == 0 {
				fmt.Println("Killing the entire glob state")
				globState = make(map[string]fsm.State)
			}

			if isMaster(PeerList, idInt, masterWaitDone) || (len(p.New) > 0 && masterWaitDone) { //There is a new node on network
				newInt, _ := strconv.Atoi(p.New)
				fmt.Println("NEW NODE!")
				fmt.Println(globState)
				for potentialGhost, _ := range globState {
					potentialGhostInt, _ := strconv.Atoi(potentialGhost)
					if potentialGhostInt == -newInt {
						fmt.Println("IM MASTER AND I CAN SEE both lift and ghost version!!!!!!!!")

						fmt.Println("delegating caborders")
						for f := 0; f < numFloors; f++ {
							if globState[potentialGhost].ExeOrders[f*3+int(io.BT_Cab)] {
								assignedOrderTx <- fsm.Order{io.ButtonEvent{f, io.BT_Cab}, newInt}
							}
						}
						fmt.Println("killing ghostie")

						delete(globState, potentialGhost)
						globStateTx <- globState
					}
				}
				//Inform the new node about the global state
				fmt.Println("Informing a new node about the global state")
				globStateTx <- globState
			}
			// Ensures that no orders are lost
			if isMaster(PeerList, idInt, masterWaitDone) && len(p.Lost) > 0 && len(PeerList) > 0 { // Network is up, but someone is lost
				for i := 0; i < len(p.Lost); i++ {
					fmt.Println("Lost a lift from network")

					fmt.Println("removing all non cab orders + delegate them to master")
					for f := 0; f < numFloors; f++ {
						if globState[p.Lost[i]].ExeOrders[f*3+int(io.BT_HallUp)] {
							//TODO DELEGATE UP ORDERS
							orderID, _ := strconv.Atoi(PeerList[0])
							n_fsm_orderCh <- fsm.Order{io.ButtonEvent{f, io.BT_HallUp}, orderID}
							globState[p.Lost[i]].ExeOrders[f*3+int(io.BT_HallUp)] = false // remove up orders
						}

						if globState[p.Lost[i]].ExeOrders[f*3+int(io.BT_HallDown)] {
							//TODO DELEGATE DOWN ORDERS
							orderID, _ := strconv.Atoi(PeerList[0])
							n_fsm_orderCh <- fsm.Order{io.ButtonEvent{f, io.BT_HallDown}, orderID}
							globState[p.Lost[i]].ExeOrders[f*3+int(io.BT_HallDown)] = false // remove down order
						}
					}
					//Create ghost state
					ghostID := "-" + p.Lost[i]
					globState[ghostID] = globState[p.Lost[i]]
					delete(globState, p.Lost[i]) // delete regular state
				}
				n_od_globstateCh <- globState
				globStateTx <- globState
			}
			fmt.Println(globState)

		case a := <-globStateRx:
			fmt.Println("Recieved globstate")

			// // Regain state?
			// for key, _ := range globState {
			// 	if key == strconv.Itoa(-idInt) {
			// 		fmt.Printf("Found backup of my state")

			// 		for f := 0; f < numFloors; f++ { // regain backup, take cab orders
			// 			if globState[key].ExeOrders[f*3+int(io.BT_Cab)] {
			// 				n_fsm_orderCh <- fsm.Order{io.ButtonEvent{f, io.BT_Cab}, idInt}
			// 			}
			// 		}
			// 		//Should not be done as this node is not master v
			// 		//fmt.Println("THERE IS A GHOST!!!")
			// 		// delete(globState, strconv.Itoa(-idInt))
			// 		// globStateTx <- globState
			// 	}
			// }

			//NB, master now sends out glob state to port and saves same glob state from port
			if !isMaster(PeerList, idInt, masterWaitDone) {
				globState = a
				n_od_globstateCh <- globState
			}
			fmt.Println(globState)

		case a := <-unassignedOrderRx:
			n_od_orderCh <- a

		case a := <-fsm_n_stateCh:
			localStateTx <- a
		case a := <-fsm_n_orderCh:
			n_od_orderCh <- a //send order to master
			unassignedOrderTx <- a
		//case a := <-countRx:
		//idPeer := a.ID
		// if isMaster(PeerList, idPeer) {
		// 	count_glob = a.Iter // Every nodes backups masters state
		// }
		case a := <-localStateRx: // recieved local state from any lift
			if isMaster(PeerList, idInt, masterWaitDone) {
				fmt.Println("UPDATING THE GLOBAL STATE")
				globState[strconv.Itoa(a.ID)] = a // update global state
				n_od_globstateCh <- globState     // send out global state on network
				globStateTx <- globState
			}

		// case a := <-countCh: // LOCAL message only heard on local computer
		// 	count_glob = a

		case a := <-assignedOrderRx:
			if a.ID == idInt {
				n_fsm_orderCh <- a
			}
		case a := <-od_n_orderCh:
			if isMaster(PeerList, idInt, masterWaitDone) {
				assignedOrderTx <- a
				if a.ID == idInt {
					n_fsm_orderCh <- a
				}
			}
			//else { send ut på nettet }
			//send order to master

			//add case for incoming message from master with new orders, send to fsm
		}
	}
}
