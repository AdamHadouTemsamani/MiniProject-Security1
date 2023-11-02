package main

import (
	"bufio"
	"context"
	"fmt"
	"sync"
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

	fieldSize int //The field size of the secret
	peerSize  int //How many peers we are communicating with.

	numberOfMessages int   //How many messages you have received.
	messagesSent     int   //How many messages you have sent.
	receivedMessages []int //An array with the messages you have received.
}

func main() {

	/* Setting up ports and context */
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5050 //Setting up your own port.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() //Ends the connection when it has finished.

	//Peer, such as Bob, Alice, Eve and Hostpital.
	p := &peer{
		id:               ownPort,
		clients:          make(map[int32]ping.SendSharesClient),
		ctx:              ctx,
		fieldSize:        514229,         //Prime in fibbonacci's sequence
		peerSize:         3,              //Number of peers in the system, excluding hospital
		numberOfMessages: 0,              //At the beginning you haven't received any messages.
		messagesSent:     0,              //Number of messages sent, which is zero.
		receivedMessages: make([]int, 3), //Received shares.
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}

	//Create a TLS server from the certificates
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
		if port == p.id {
			continue
		}

		fmt.Printf("Trying to dial: %v\n", port)
		clientCertificate, err := credentials.NewClientTLSFromFile("certificate/server.crt", "")
		if err != nil {
			log.Fatalf("Big error:  %s", err)
		}

		//Create a client that communicate through TLS
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
			if secret < 0 || secret > 500000 { //Make sure that the secret is within the field size
				fmt.Println("Please enter a number between 0 and 500.000")
			} else {
				fmt.Printf("You have chosen the secret: %d \n", secret)
				p.ShareSecret(int(secret))
			}
		}
	}
}

// Share Secrets between each of the parties
func (p *peer) ShareSecret(secret int) {
	shares := splitShare(secret, p.fieldSize) //Get shares, which is saved in an array/slice
	fmt.Printf("These are my shares: %d \n", shares)

	//We only send the shares if we haven't sent any.
	if p.messagesSent == 0 {
		shareId := 0
		for id, _ := range p.clients {
			if id == 5050 || id == (p.id) { //Don't send if you are hospital and don't send it to yourself
				continue
			}
			fmt.Printf("I am sending (%d) to peer: (%d) \n", shares[shareId], id)
			p.BroadcastToPeers(shares[shareId], id)
			shareId++        //So we get the next share for the array.
			p.messagesSent++ //Increment messagesSent
		}

		//After sending the two first shares to your peers, keep the last one to yourself.
		fmt.Printf("I am keeping (%d) for myself \n", shares[shareId])
		p.receivedMessages[p.numberOfMessages] = shares[shareId]
		p.numberOfMessages++
	}

	//If I have received shares all shares, broadcast it to the hospital.
	if (p.numberOfMessages) == 3 {
		var sumOfShares int
		for _, share := range p.receivedMessages {
			sumOfShares += share //Create sum of shares
		}
		p.BroadcastToHospital(sumOfShares) //Broadcast to hospital.

	}
}

// GRPC method used to communicate between the peers, when sending shares.
// It takes a "share" as an argument and returns an "acknoledgement".
func (p *peer) Send(ctx context.Context, share *ping.Share) (*ping.Acknoledgement, error) {
	var a sync.Mutex
	a.Lock()
	s := share.Message //Create share

	//Received 2 shares and we are not the hospital.
	if p.numberOfMessages == 2 && p.id != 5050 {
		p.receivedMessages[p.numberOfMessages] = int(s)
		p.numberOfMessages = 0
		go func() {
			time.Sleep(time.Millisecond * 3)
			p.CombineSharesAndSend() //Send to hospital, since we have now received all three shares.
		}()
		return &ping.Acknoledgement{Message: s}, nil
	}

	//If you are the hospital
	if p.id == 5050 {
		p.receivedMessages[p.numberOfMessages] = int(s)
		p.numberOfMessages++
		fmt.Printf("I have received sum number: %d, which is: %d.\n", p.numberOfMessages, s)
		if p.numberOfMessages == 2 { //Received all shares
			time.Sleep(time.Second)
			var sumOfShares int
			for _, share := range p.receivedMessages {
				sumOfShares += share
			}
			sumOfShares = sumOfShares % p.fieldSize
			fmt.Println("I have received sums from all peers.")
			fmt.Printf("The final sum is %d.\n", sumOfShares)
			p.numberOfMessages = 0
		}
	}

	a.Unlock()
	//If you are the last peer to send a share, then broadcast message to hospital.
	if p.messagesSent == 3 {
		go func() {
			time.Sleep(time.Millisecond * 5)
			p.CombineSharesAndSend() //Send to hospital.
		}()
		p.messagesSent = 0
		p.numberOfMessages = 0 //Reset the number of messages so that the protocol can be run again.
		return &ping.Acknoledgement{Message: s}, nil

	}
	if p.id != 5050 { //If you are not the hopsital, receive the message.
		p.receivedMessages[p.numberOfMessages] = int(s)
		p.numberOfMessages++
		return &ping.Acknoledgement{Message: s}, nil
	}
	return &ping.Acknoledgement{Message: s}, nil
}

// Combine the shares and send them to hospital.
func (p *peer) CombineSharesAndSend() {
	var shares int
	for _, share := range p.receivedMessages {
		fmt.Printf("I am peer (%d) and the share is (%d) \n", p.id, share)
		shares += share //Sum the three received shares.
	}
	shares = shares % p.fieldSize //Created combined share

	fmt.Printf("I am sending the sum %d to the hospital. \n", shares)
	go func() {
		p.BroadcastToHospital(shares) //Broadcast share to hospital.
	}()
}

// Broadcast the sum of shares to the hospital
func (p *peer) BroadcastToHospital(sumOfShares int) {
	hospital := p.clients[5050]                       //The hospital peer
	share := &ping.Share{Message: int32(sumOfShares)} //Create share message

	time.Sleep(time.Second * 1)
	_, err := hospital.Send(p.ctx, share)
	if err != nil {
		log.Print("Something went wrong! Method: BroadcastToHospital()")
	}
	p.receivedMessages = make([]int, 3) //Remove shares from array
}

// Broadcast/Send the shares to the hospital
func (p *peer) BroadcastToPeers(secret int, index int32) {
	client := p.clients[index]                   //Get peer by index
	share := &ping.Share{Message: int32(secret)} //Create share message
	_, err := client.Send(p.ctx, share)
	if err != nil {
		log.Print("Something went wrong! Method: BroadcastToPeers()")
	}
}

// Make three shares using a circular group using a prime number.
func splitShare(secret int, fieldSize int) []int {
	rand.Seed(time.Now().UnixNano())
	share1 := rand.Intn(fieldSize - 1)
	share2 := rand.Intn(fieldSize - 1)
	finalShare := (secret - ((share1 + share2) % fieldSize)) % fieldSize //Compute the last share
	if finalShare < 0 {
		finalShare = finalShare + fieldSize //If the finalshare is less than 0 we compute the actual modulo
	}
	return []int{share1, share2, finalShare}
}
