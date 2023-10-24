package main

import (
	ping "Mini_Project_2/proto"
	"bufio"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
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

	var ownKey int
	field := 514229 //Prime in fibbonacci's sequence
	numOfPeers := 3

	if ownPort != 5050 {
		ownKey = rand.Intn(field - 1)
	} else {
		ownKey = 0 //Hospital does not have private key, to avoid problems it is set to 0.
	}

	//Peer, such as Bob, Alice, Eve and Hostpital.
	p := &peer{
		id:            ownPort,
		amountOfPings: make(map[int32]int32),
		clients:       make(map[int32]ping.SendSharesClient),
		ctx:           ctx,

		privateKey:       ownKey,
		fieldSize:        field,
		peerSize:         numOfPeers,
		numberOfMessages: 0, //At the beginning you haven't received any messages.
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	ping.RegisterSendSharesServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := int32(5050) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}

		defer conn.Close()
		c := ping.NewSendSharesClient(conn)
		p.clients[port] = c
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if ownPort == 5050 { //If you are the hosptital
			fmt.Print("Welcome to the service!")
			fmt.Print("To share a secret with the hospital, enter a number between 0 and 500.000")
			secret, err := strconv.ParseInt(scanner.Text(), 10, 32)
			if err != nil {
				fmt.Print("Please enter a number!")
			}
			if secret < 0 || secret > 500000 {
				fmt.Print("Please enter a number between 0 and 500.000")
			} else {
				p.ShareSecret(int(secret))
			}
		} else { //If you are on the clients/peer
			fmt.Print("Welcome to the service!")
			fmt.Print("You are the hospital. Please await, while the peers are sending you their secrets.")
		}
	}
}

func (p *peer) SendShare(ctx context.Context, share *ping.Share) (*ping.Acknoledgement, error) {
	s := share.Message
	if p.numberOfMessages == 2 && p.id != 5050 { //Received 2 chunks and we are not the hospital.
		fmt.Printf("I received the chunk: %d", s)
		p.numberOfMessages = 0 //Reset the number of messages so that the protocol can be run again.
		return &ping.Acknoledgement{Message: s}, nil
	}
	p.numberOfMessages++
	p.receivedMessages = append(p.receivedMessages, int(s))

	//Check if a peer has received three shares
	if len(p.receivedMessages) == 3 {
		go func() {
			time.Sleep(time.Millisecond * 3)
			p.CombineSharesAndSend()
		}()
	}
	return &ping.Acknoledgement{Message: s}, nil
}

// Combine the shares and send them to hospital.
func (p *peer) CombineSharesAndSend() {
	var shares int
	for _, share := range p.receivedMessages {
		shares += share
	}
	shares = shares % p.fieldSize //Created combined share
	if p.id == 5050 {
		p.BroadcastShares(shares)
	} else {
		p.BroadcastToHospital(shares)
	}
}

// If you are hospital broadcast result to peers.
func (p *peer) BroadcastShares(shares int) {

	for id, _ := range p.clients {
		if id == (p.id - 5050) {
			continue
		}
		p.BroadcastToPeers(shares, id)
	}
}

// Send shares (secret) to hospital
func (p *peer) BroadcastToHospital(sumOfShares int) {
	hospital := p.clients[0]
	share := &ping.Share{Message: int32(sumOfShares)}
	fmt.Printf("Sending share (%d) to hospital (%d)", sumOfShares, hospital)
	ack, err := hospital.SendShares(p.ctx, share)
	if err != nil {
		log.Print("Something went wrong!")
	}
	fmt.Printf("%v has received the share %v", 0, ack.Message)

}

// Send shares (secret) to peer
func (p *peer) BroadcastToPeers(secret int, index int32) {
	client := p.clients[index]
	share := &ping.Share{Message: int32(secret)}
	fmt.Printf("Sending secret (%d) to peer (%d)", secret, client)
	ack, err := client.SendShares(p.ctx, share)
	if err != nil {
		log.Print("Something went wrong!")
	}
	fmt.Printf("%v has received the share %v", 0, ack.Message)
}

func (p *peer) ShareSecret(secret int) {
	shares := splitShare(secret, p.peerSize, p.fieldSize)
	shareId := 0
	for id, _ := range p.clients {
		if id == 0 || id == (p.id-5050) { //Don't send if you are hospital and don't send it to yourself
			continue
		}
		p.BroadcastToPeers(shares[shareId], id)
		shareId++
	}

	//Keep the last of three shares to yourself, which is why we set shareId to 0
	p.receivedMessages = append(p.receivedMessages, shares[shareId])
	if len(p.receivedMessages) == 3 {
		var sumOfShares int
		for _, share := range p.receivedMessages {
			sumOfShares += share
		}

		p.BroadcastToHospital(sumOfShares)
		p.receivedMessages = p.receivedMessages[:0] //Empty array, but keep allocated memory
	}

}

// Makes shares suing a circular group
// Make field size prime number
// i guess secret is random number in field
func splitShare(secret int, N int, fieldSize int) []int {
	array := make([]int, N)
	for i := 1; i < N-1; i++ {
		array = append(array, rand.Intn(fieldSize))
	}

	/* Compute sum of shares to get the last share */
	var sum int
	for i := 0; i < len(array); i++ {
		sum += array[i]
	}

	array = append(array, (secret-sum)%fieldSize)
	return array
}
