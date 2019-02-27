package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"strconv"

	//"github.com/kr/pretty"
	"golang.org/x/net/context"
	"googlemaps.github.io/maps"
	"gopkg.in/njern/gonexmo.v2"
	"github.com/martinlindhe/notify"
)

var (
	apiKey                   = flag.String("key", "", "Use your API Key for using Google Maps API.")
	clientID                 = flag.String("client_id", "", "ClientID for Maps for Work API access.")
	signature                = flag.String("signature", "", "Signature for Maps for Work API access.")
	origin                   = flag.String("origin", "Sydney", "The address or textual latitude/longitude value from which you wish to calculate directions.")
	destination              = flag.String("destination", "Perth", "The address or textual latitude/longitude value from which you wish to calculate directions.")
	mode                     = flag.String("mode", "driving", "The travel mode for this directions request.")
	departureTime            = flag.String("departure_time", strconv.FormatInt(time.Now().UTC().UnixNano(), 10), "The depature time for transit mode directions request.")
	arrivalTime              = flag.String("arrival_time", "", "The arrival time for transit mode directions request.")
	waypoints                = flag.String("waypoints", "", "The waypoints for driving directions request, | separated.")
	alternatives             = flag.Bool("alternatives", false, "Whether the Directions service may provide more than one route alternative in the response.")
	avoid                    = flag.String("avoid", "", "Indicates that the calculated route(s) should avoid the indicated features, | separated.")
	language                 = flag.String("language", "", "Specifies the language in which to return results.")
	units                    = flag.String("units", "", "Specifies the unit system to use when returning results.")
	region                   = flag.String("region", "", "Specifies the region code, specified as a ccTLD (\"top-level domain\") two-character value.")
	transitMode              = flag.String("transit_mode", "", "Specifies one or more preferred modes of transit, | separated. This parameter may only be specified for transit directions.")
	transitRoutingPreference = flag.String("transit_routing_preference", "", "Specifies preferences for transit routes.")
	iterations               = flag.Int("iterations", 1, "Number of times to make API request.")
	contactNo               = flag.String("contact_no", "", "contact no for notification")
	trafficModel             = flag.String("traffic_model", "pessimistic", "Specifies traffic prediction model when request future directions. Valid values are optimistic, best_guess, and pessimistic. Optional.")
)

func usageAndExit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Println("Flags:")
	flag.PrintDefaults()
	os.Exit(2)
}

func check(err error) {
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}
}

func fmtDuration(d time.Duration) string {
    d = d.Round(time.Minute)
    h := d / time.Hour
    d -= h * time.Hour
    m := d / time.Minute
    return fmt.Sprintf("%02dh:%02dm", h, m)
}

func main() {
	flag.Parse()
    nexmoClient, _ := nexmo.NewClient("", "use your key for nexo client")
	var client *maps.Client
	var err error
	if *apiKey != "" {
		client, err = maps.NewClient(maps.WithAPIKey(*apiKey), maps.WithRateLimit(2))
	} else if *clientID != "" || *signature != "" {
		client, err = maps.NewClient(maps.WithClientIDAndSignature(*clientID, *signature))
	} else {
		usageAndExit("Please specify an API Key, or Client ID and Signature.")
	}
	check(err)

	r := &maps.DirectionsRequest{
		Origin:        *origin,
		Destination:   *destination,
		DepartureTime: *departureTime,
		ArrivalTime:   *arrivalTime,
		Alternatives:  *alternatives,
		Language:      *language,
		Region:        *region,
	}

	lookupMode(*mode, r)
	lookupUnits(*units, r)
	lookupTransitRoutingPreference(*transitRoutingPreference, r)
	lookupTrafficModel(*trafficModel, r)

	if *waypoints != "" {
		r.Waypoints = strings.Split(*waypoints, "|")
	}

	if *avoid != "" {
		for _, a := range strings.Split(*avoid, "|") {
			switch a {
			case "tolls":
				r.Avoid = append(r.Avoid, maps.AvoidTolls)
			case "highways":
				r.Avoid = append(r.Avoid, maps.AvoidHighways)
			case "ferries":
				r.Avoid = append(r.Avoid, maps.AvoidFerries)
			default:
				log.Fatalf("Unknown avoid restriction %s", a)
			}
		}
	}
	if *transitMode != "" {
		for _, t := range strings.Split(*transitMode, "|") {
			switch t {
			case "bus":
				r.TransitMode = append(r.TransitMode, maps.TransitModeBus)
			case "subway":
				r.TransitMode = append(r.TransitMode, maps.TransitModeSubway)
			case "train":
				r.TransitMode = append(r.TransitMode, maps.TransitModeTrain)
			case "tram":
				r.TransitMode = append(r.TransitMode, maps.TransitModeTram)
			case "rail":
				r.TransitMode = append(r.TransitMode, maps.TransitModeRail)
			}
		}
	}

	if *iterations == 1 {
		//routes, waypoints, err := client.Directions(context.Background(), r)
		routes, _, err := client.Directions(context.Background(), r)
		check(err)

		//pretty.Println(waypoints)
		//pretty.Println(routes)
		//pretty.Println(routes[0].Summary)
		//pretty.Println(fmtDuration(routes[0].Legs[0].Duration))

		message := &nexmo.SMSMessage{
	From:           "KodeFuPandas",
    To:              *contactNo,
	Type:            nexmo.Text,
	Text:            "USE " + routes[0].Summary + " FOR HEADING HOME \n-KodeFuPandas ,\n",
	//ClientReference: "" + strconv.FormatInt(time.Now().Unix(), 10),
	Class:           nexmo.Standard,
	}

	messageResponse, err := nexmoClient.SMS.Send(message)
	fmt.Println(messageResponse)
	notify.Alert("CHOOSE:" + routes[0].Summary + ".It will take " + fmtDuration(routes[0].Legs[0].DurationInTraffic), "Optimal path for HOME", "some text", "path/to/icon.png")







	//	sum := routes["summary"]
	//	fmt.Println(sum)

	} else {
		done := make(chan iterationResult)
		for i := 0; i < *iterations; i++ {
			go func(i int) {
				startTime := time.Now()
				_, _, err := client.Directions(context.Background(), r)
				done <- iterationResult{
					fmt.Sprintf("Iteration %2d: round trip %.2f seconds", i, float64(time.Now().Sub(startTime))/1000000000),
					err,
				}
			}(i)
		}

		for i := 0; i < *iterations; i++ {
			result := <-done
			if err != nil {
				fmt.Printf("error: %+v\n", result.err)
			} else {
				fmt.Println(result.result)
			}
		}
	}



// Test if it works by retrieving your account balance
//balance, err := nexmoClient.Account.GetBalance()

// Send an SMS
// See https://docs.nexmo.com/index.php/sms-api/send-message for details.

}

type iterationResult struct {
	result string
	err    error
}

func lookupMode(mode string, r *maps.DirectionsRequest) {
	switch mode {
	case "driving":
		r.Mode = maps.TravelModeDriving
	case "walking":
		r.Mode = maps.TravelModeWalking
	case "bicycling":
		r.Mode = maps.TravelModeBicycling
	case "transit":
		r.Mode = maps.TravelModeTransit
	case "":
		// ignore
	default:
		log.Fatalf("Unknown mode '%s'", mode)
	}
}

func lookupUnits(units string, r *maps.DirectionsRequest) {
	switch units {
	case "metric":
		r.Units = maps.UnitsMetric
	case "imperial":
		r.Units = maps.UnitsImperial
	case "":
		// ignore
	default:
		log.Fatalf("Unknown units '%s'", units)
	}
}

func lookupTransitRoutingPreference(transitRoutingPreference string, r *maps.DirectionsRequest) {
	switch transitRoutingPreference {
	case "fewer_transfers":
		r.TransitRoutingPreference = maps.TransitRoutingPreferenceFewerTransfers
	case "less_walking":
		r.TransitRoutingPreference = maps.TransitRoutingPreferenceLessWalking
	case "":
		// ignore
	default:
		log.Fatalf("Unknown transit routing preference %s", transitRoutingPreference)
	}
}

func lookupTrafficModel(trafficModel string, r *maps.DirectionsRequest) {
	switch trafficModel {
	case "optimistic":
		r.TrafficModel = maps.TrafficModelOptimistic
	case "best_guess":
		r.TrafficModel = maps.TrafficModelBestGuess
	case "pessimistic":
		r.TrafficModel = maps.TrafficModelPessimistic
	case "":
		// ignore
	default:
		log.Fatalf("Unknown traffic mode %s", trafficModel)
	}
}
