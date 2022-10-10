// Command client creates Begin/Invoke packet with given parameters, and send it to the specified address.
// By default, it sends MAP cancelLocation. The parameters in the lower layers(SCTP/M3UA/SCCP) cannot be
// specified from command-line arguments. Update this source code itself to update them.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ishidawataru/sctp"
	"github.com/Amrrx/go-m3ua"
	m3params "github.com/Amrrx/go-m3ua/messages/params"
	"github.com/Amrrx/go-sccp"
	"github.com/Amrrx/go-sccp/params"
	"github.com/Amrrx/go-sccp/utils"
	"github.com/Amrrx/go-tcap"
)
// "800791180758002431810100820791180758188106"
// 800712345678912534810100820712345678912534 -> Modified
func sendRoutingInfoForSM() []byte {
	var (
		otid    = flag.Int("otid", 0x11111111, "Originating Transaction ID in uint32.")
		opcode  = flag.Int("opcode", 45, "Operation Code in int.")
		// Tag 80 = IMSDIN , 07 Size of message, 1 - 888 IMSDIN
		// Tag 81 = sm-RP-PRI, 01, FF = True / 00 = False
		// Tag 82 = Service Centre Address, 07 Size of message, 1 - 999 IMSDIN
		payload = flag.String("payload", "8007123456789128888101FF820712345678912999", "Hex representation of the payload")
	)
	flag.Parse()
	p, err := hex.DecodeString(*payload)
	if err != nil {
		log.Fatal(err)
	}
	tcapBytes, err := tcap.NewBeginInvokeWithDialogue(
		uint32(*otid),             // OTID
		tcap.DialogueAsID,         // DialogueType
		tcap.SendRoutingInfoForSM, // ACN
		3,                         // ACN Version
		113,                       // Invoke Id
		*opcode,                   // OpCode
		p,                         // Payload
	).MarshalBinary()
	if err != nil {
		log.Fatal(err)
	}
	return tcapBytes
}

func sendCancelLocation() []byte {
	var (
		otid    = flag.Int("otid", 0x11111111, "Originating Transaction ID in uint32.")
		opcode  = flag.Int("opcode", 3, "Operation Code in int.")
		payload = flag.String("payload", "040800010121436587f9", "Hex representation of the payload")
	)
	flag.Parse()
	p, err := hex.DecodeString(*payload)
	if err != nil {
		log.Fatal(err)
	}
	//tcap.Parse()
	tcapBytes, err := tcap.NewBeginInvokeWithDialogue(
		uint32(*otid),                    // OTID
		tcap.DialogueAsID,                // DialogueType
		tcap.LocationCancellationContext, // ACN
		3,                                // ACN Version
		0,                                // Invoke Id
		*opcode,                          // OpCode
		p,                                // Payload
	).MarshalBinary()
	if err != nil {
		log.Fatal(err)
	}
	return tcapBytes
}



func main() {
	var (
		addr = flag.String("addr", "127.0.0.1:2905", "Remote IP and Port to connect to.")
	)

	targetMessage := sendRoutingInfoForSM()

	// create *Config to be used in M3UA connection
	m3config := m3ua.NewConfig(
		0x11111111,              // OriginatingPointCode
		0x22222222,              // DestinationPointCode
		m3params.ServiceIndSCCP, // ServiceIndicator
		0,                       // NetworkIndicator
		0,                       // MessagePriority
		1,                       // SignalingLinkSelection
	).EnableHeartbeat(5*time.Second, 100*time.Second)

	// setup SCTP peer on the specified IPs and Port.
	raddr, err := sctp.ResolveSCTPAddr("sctp", *addr)
	if err != nil {
		log.Fatalf("Failed to resolve SCTP address: %s", err)
	}

	// setup underlying SCTP/M3UA connection first
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m3conn, err := m3ua.Dial(ctx, "m3ua", nil, raddr, m3config)
	if err != nil {
		log.Fatal(err)
	}

	cdPA, err := utils.StrToSwappedBytes("817085004213", "0")
	if err != nil {
		log.Fatal(err)
	}
	cgPA, err := utils.StrToSwappedBytes("817085811860", "0")
	if err != nil {
		log.Fatal(err)
	}

	// create UDT message with CdPA, CgPA and payload
	udt, err := sccp.NewUDT(
		0,    // Protocol Class
		true, // Message handling
		params.NewPartyAddress( // CalledPartyAddress: 1234567890123456
			0x12, 0, 6, 0x00, // Indicator, SPC, SSN, TT
			0x01, 0x02, 0x04, // NP, ES, NAI
			cdPA, // GlobalTitleInformation
		),
		params.NewPartyAddress( // CallingPartyAddress: 9876543210
			0x12, 0, 8, 0x00, // Indicator, SPC, SSN, TT
			0x01, 0x02, 0x04, // NP, ES, NAI
			cgPA, // GlobalTitleInformation
		),
		targetMessage,
	).MarshalBinary()
	if err != nil {
		log.Fatal(err)
	}

	// send once
	if _, err := m3conn.Write(udt); err != nil {
		log.Fatal(err)
	}


	// A new ticker for the heartbeats
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	for {
		ticker := time.NewTicker(5 * time.Second)
		select {
		case sig := <-sigCh:
			log.Printf("Got signal: %v, exitting...", sig)
			ticker.Stop()
			os.Exit(1)
		case <-ticker.C:
			log.Println("Beat....")
			// if _, err := m3conn.Write(udt); err != nil {
			// 	log.Fatal(err)
			// }
		}
	}
	
}
