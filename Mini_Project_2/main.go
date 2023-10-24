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
	ping.UnimplementedPingServer
	id            int32
	amountOfPings map[int32]int32
	clients       map[int32]ping.PingClient
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

	if ownPort != 5000 {
		ownKey = rand.Intn(field - 1)
	} else {
		ownKey = 0 //Hospital does not have private key, to avoid problems it is set to 0.
	}

	//Peer, such as Bob, Alice, Eve and Hostpital.
	p := &peer{
		id:            ownPort,
		amountOfPings: make(map[int32]int32),
		clients:       make(map[int32]ping.PingClient),
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
	ping.RegisterPingServer(grpcServer, p)

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
		c := ping.NewPingClient(conn)
		p.clients[port] = c
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.sendPingToAll()
	}
}

func (p *peer) Ping(ctx context.Context, req *ping.Request) (*ping.Reply, error) {
	id := req.Id
	p.amountOfPings[id] += 1

	rep := &ping.Reply{Amount: p.amountOfPings[id]}
	return rep, nil
}

func (p *peer) sendPingToAll() {
	request := &ping.Request{Id: p.id}
	for id, client := range p.clients {
		reply, err := client.Ping(p.ctx, request)
		if err != nil {
			fmt.Println("something went wrong")
		}
		fmt.Printf("Got reply from id %v: %v\n", id, reply.Amount)
	}
}

func (p *peer) SendShare(ctx context.Context, share *ping.Share) (*ping.Acknoledgement, error) {
	//shares := splitShare(p.privateKey, p.peerSize, p.fieldSize)
	s := share.Message

	if p.numberOfMessages == 2 && p.id != 5000 { //Received 2 chunks and we are not the hospital.
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
	if p.id == 5000 {
		p.BroadcastShares(shares)
	} else {
		p.BroadcastToHospital(shares)
	}
}

// If you are hospital broadcast result to peers.
func (p *peer) BroadcastShares(shares int) {

	for id, _ := range p.clients {
		if id == (p.id - 5000) {
			continue
		}
		p.BroadcastToPeers(shares, id)
	}
}

// Send shares (secret) to hospital
func (p *peer) BroadcastToHospital(secretshare int) {
	hospital := p.clients[0]
	share := &ping.Share{Message: int32(secretshare)}
	fmt.Printf("Sending share (%d) to hospital (%d)", secretshare, hospital)
	ack, err :=

}

func (p *peer) BroadcastToPeers(share int, id int32) {
	i := id - 5000 //Index of client
	client := p.clients[i]
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
