package main

import (
	"bufio"
	"context"
	"fmt"
	"time"

	ping "Mini_Project_2/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
)

type peer struct {
	ping.UnimplementedSendSharesServer
	id            int32
	amountOfPings map[int32]int32
	clients       map[int32]ping.SendSharesClient
	ctx           context.Context

	privateKey int
	fieldSize  int
	peerSize   int

	numberOfMessages int
	receivedMessages []int
}

func main() {

	/* Setting up ports and context */
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5050 //Setting up your own port.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() //Ends the connection when it has finished.

	field := 514229 //Prime in fibbonacci's sequence

	//Peer, such as Bob, Alice, Eve and Hostpital.
	p := &peer{
		id:            ownPort,
		amountOfPings: make(map[int32]int32),
		clients:       make(map[int32]ping.SendSharesClient),
		ctx:           ctx,

		privateKey:       0,
		fieldSize:        field,
		peerSize:         3, //Number of peers in the system, excluding hospital
		numberOfMessages: 0, //At the beginning you haven't received any messages.
		receivedMessages: make([]int, 3),
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}

	serverCertificate, err := credentials.NewServerTLSFromFile("certificate/server.crt", "certificate/priv.key")
	if err != nil {
		log.Fatalf("Big error:  %s", err)
	}
	grpcServer := grpc.NewServer(grpc.Creds(serverCertificate))
	ping.RegisterSendSharesServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to serve %v", err)
		}
	}()

	for i := 0; i <= 3; i++ {
		port := int32(5050) + int32(i)

		if port == ownPort {
			continue
		}

		fmt.Printf("Trying to dial: %v\n", port)
		clientCertificate, err := credentials.NewClientTLSFromFile("certificate/server.crt", "")
		if err != nil {
			log.Fatalf("Big error:  %s", err)
		}

		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithTransportCredentials(clientCertificate), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := ping.NewSendSharesClient(conn)
		p.clients[port] = c
	}

	scanner := bufio.NewScanner(os.Stdin)
	if ownPort == 5050 { //If you are the hosptital
		fmt.Println("Welcome to the service!")
		fmt.Println("You are the hospital. Please wait, while the peers are sending you their secrets.")
		for scanner.Scan() {
			fmt.Println("Please wait for the peers!")
			fmt.Printf("Your id: %d \n", ownPort)
		}

	} else { //If you are on the clients/peer
		fmt.Println("Welcome to the service!")
		fmt.Println("To share a secret with the hospital, enter a number between 0 and 500.000")
		for scanner.Scan() {
			secret, err := strconv.ParseInt(scanner.Text(), 10, 32)
			if err != nil {
				fmt.Println("Please enter a number!")
			}
			if secret < 0 || secret > 500000 {
				fmt.Println("Please enter a number between 0 and 500.000")
			} else {
				fmt.Printf("You have chosen the secret: %d \n", secret)
				p.privateKey = int(secret)
				p.ShareSecret(int(secret))
			}
		}
	}
}

func (p *peer) ShareSecret(secret int) {
	shares := splitShare(secret, p.peerSize, p.fieldSize) //Get shares, which is saved in an array
	fmt.Println(shares)

	shareId := 0
	for id, _ := range p.clients {
		fmt.Printf("This is the id: %d \n", id)
		if id == 5050 || id == (p.id) { //Don't send if you are hospital and don't send it to yourself
			continue
		}
		p.BroadcastToPeers(shares[shareId], id)
		shareId++
	}

	//Keep the last of three shares to yourself, which is why we set shareId to 0
	p.receivedMessages[p.numberOfMessages] = shares[shareId]
	p.numberOfMessages++

	time.Sleep(time.Millisecond * 5)
	if len(p.receivedMessages) == 3 {
		var sumOfShares int
		for _, share := range p.receivedMessages {
			sumOfShares += share
		}

		p.BroadcastToHospital(sumOfShares)
		p.receivedMessages = p.receivedMessages[:0] //Empty array, but keep allocated memory
	}
}

func (p *peer) SendShare(ctx context.Context, share *ping.Share) (*ping.Acknoledgement, error) {
	s := share.Message
	if p.numberOfMessages == 2 && p.id != 5050 { //Received 2 chunks and we are not the hospital.
		fmt.Printf("I received the final share: %d", s)
		p.numberOfMessages++

		//We should now have received three shares

		if len(p.receivedMessages) == 3 {
			go func() {
				time.Sleep(time.Millisecond * 3)
				p.CombineSharesAndSend()
			}()
		}
		p.numberOfMessages = 0 //Reset the number of messages so that the protocol can be run again.
		return &ping.Acknoledgement{Message: s}, nil
	}
	fmt.Printf("I have received the share: %d", s)
	p.receivedMessages[p.numberOfMessages] = int(s)
	p.numberOfMessages++
	return &ping.Acknoledgement{Message: s}, nil
}

// Combine the shares and send them to hospital.
func (p *peer) CombineSharesAndSend() {
	var shares int
	for _, share := range p.receivedMessages {
		shares += share //Sum the three received shares.
	}
	shares = shares % p.fieldSize //Created combined share

	if p.id == 5050 {
		p.BroadcastShares(shares) //Broadcast shares to peers.
	} else {
		p.BroadcastToHospital(shares) //Broadcast share to hospital.
	}
}

// If you are hospital broadcast result to peers.
func (p *peer) BroadcastShares(shares int) {

	for id, _ := range p.clients {
		if id == p.id {
			continue
		}
		p.BroadcastToPeers(shares, id)
	}
}

// Send shares (secret) to hospital
func (p *peer) BroadcastToHospital(sumOfShares int) {
	hospital := p.clients[0]                          //The hospital peer is the first index
	share := &ping.Share{Message: int32(sumOfShares)} //Create share message

	fmt.Printf("Sending share (%d) to hospital (%d)", sumOfShares, hospital)
	ack, err := hospital.SendShares(p.ctx, share)
	if err != nil {
		log.Print("Something went wrong! Method: BroadcastToHospital()")
	}
	fmt.Printf("%v should have received the share %v", 0, ack.Message)
}

// Send shares (secret) to peer
func (p *peer) BroadcastToPeers(secret int, index int32) {
	fmt.Printf("Index: %d \n", index)
	client := p.clients[index]                   //Get peer by index
	share := &ping.Share{Message: int32(secret)} //Create share message

	fmt.Printf("Sending secret (%d) to peer (%d)", secret, client)
	ack, err := client.SendShares(p.ctx, share)
	if err != nil {
		log.Print("Something went wrong! Method: BroadcastToPeers()")
	}
	fmt.Printf("%v should have received the share %v", 0, ack.Message)
}

// Make three shares using a circular group using a prime number.
func splitShare(secret int, N int, fieldSize int) []int {
	array := make([]int, N)
	for i := 0; i < N-1; i++ {
		rand.Seed(time.Now().UTC().UnixNano())
		randomShare := rand.Intn(fieldSize)
		array[i] = randomShare
	}

	var sum int
	for i := 0; i < len(array); i++ {
		sum += array[i] //Add the two shares, this is used to compute the last share.
	}
	array[2] = (secret - sum) % fieldSize //Compute the last share
	return array
}
