package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"time"
)

type GPX struct {
	XMLName xml.Name `xml:"gpx"`
	Track   Track    `xml:"trk"`
}

type Track struct {
	Segment Segment `xml:"trkseg"`
}

type Segment struct {
	Points []Point `xml:"trkpt"`
}

type Point struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Ele  float64 `xml:"ele"`
	Time string  `xml:"time"`
	Ext  Ext     `xml:"extensions"`
}

type Ext struct {
	Power float64 `xml:"power"`
	TPX   TPX     `xml:"gpxtpx:TrackPointExtension"`
}

type TPX struct {
	Cad int `xml:"gpxtpx:cad"`
}

type DataPoint struct {
	Lat, Lon, Ele, Power, Speed float64
	Cad                  int
	Time                 string
}

type ChainringPowerLoss struct {
	Gear  int
	Power float64
	BadGearFactor float64
}

type FTPZone struct {
	Zone    string
	Min, Max, Coefficient float64
}

type Gear struct {
	GearNumber, Chainring, Cog int
	Speed, Cadence             float64
	PowerLoss                  float64
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	R := 6371e3 // Radius of the Earth in metres
	φ1 := lat1 * math.Pi/180 // φ, λ in radians
	φ2 := lat2 * math.Pi/180
	Δφ := (lat2-lat1) * math.Pi/180
	Δλ := (lon2-lon1) * math.Pi/180

	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*
			math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := R * c // in metres
	return distance
}
// Function to parse the GPX file
func parseGPX(gpxFile string) ([]DataPoint, error) {
	// Read the file
	content, err := ioutil.ReadFile(gpxFile)
	if err != nil {
		return nil, err
	}

	var gpx GPX
	err = xml.Unmarshal(content, &gpx)
	if err != nil {
		return nil, err
	}

	// Transform the parsed data into the DataPoint struct
	var dataPoints []DataPoint
	layout := "2006-01-02T15:04:05Z" // this is an example layout, adjust it to match your timestamp format

	for i, point := range gpx.Track.Segment.Points {
		dataPoint := DataPoint{
			Lat:   point.Lat,
			Lon:   point.Lon,
			Ele:   point.Ele,
			Time:  point.Time,
			Power: point.Ext.Power,
			Cad:   point.Ext.TPX.Cad,
		}
		if i > 0 {
			prevPoint := dataPoints[i-1]
			distance := haversine(prevPoint.Lat, prevPoint.Lon, point.Lat, point.Lon) // calculate distance
			time1, _ := time.Parse(layout, point.Time) 
			time2, _ := time.Parse(layout, prevPoint.Time)
			timeDiff := time1.Sub(time2).Seconds() // calculate time difference in seconds
			if timeDiff != 0 {
				dataPoint.Speed = (distance/timeDiff) * 3.6 // convert speed from m/s to km/h
			}
		}
		dataPoints = append(dataPoints, dataPoint)
	}
	return dataPoints, nil
}



func calculateGear(speed, cadence float64, chainring int, cassette []int, tyreCircumference float64, chainRingPowerLoss []ChainringPowerLoss, ftpZones []FTPZone) Gear {
	// Define a high initial minimum difference
	minDifference := math.MaxFloat64
	var chosenGear Gear
	
	// Convert speed from km/h to m/s
	speed = speed / 3.6

	// Loop over each possible cog in the cassette
	for i, cog := range cassette {
		// Calculate the gear ratio and the difference to the required gear ratio
	// Calculate the gear ratio and the difference to the required gear ratio
if cadence == 0 || chainring == 0 || cog == 0 {
	continue // Skip this iteration if cadence, chainring, or cog is zero
}
		gearRatio := float64(chainring) / float64(cog)
		requiredGearRatio := speed / (cadence * tyreCircumference / (1000 * 60)) // the 1000 * 60 factor converts from m/min to m/s
		if math.IsNaN(gearRatio) || math.IsNaN(requiredGearRatio) {
			continue
		}
		difference := math.Abs(gearRatio - requiredGearRatio)
	//	fmt.Printf("Gear Ratio: %f, Required Gear Ratio: %f, Difference: %f\n", gearRatio, requiredGearRatio, difference)


		// If this gear is closer to the required gear ratio, choose it
		if difference < minDifference {
			minDifference = difference
			chosenGear = Gear{
				GearNumber: i, // replace -1 with i, the loop variable
				Chainring:  chainring,
				Cog:        cog,
				Speed:      speed,
				Cadence:    cadence,
			}
		}
	}

	// Calculate the power loss for this gear
	chosenGear.PowerLoss = calculatePowerLoss(chosenGear, chosenGear.Speed*3.6*float64(chosenGear.Cadence)*float64(chosenGear.Chainring)/float64(chosenGear.Cog), chainRingPowerLoss, ftpZones)
//	fmt.Printf("Chosen Gear: Gear Number - %d, Chainring - %d, Cog - %d, Speed - %f, Cadence - %f, Power Loss - %f\n", chosenGear.GearNumber, chosenGear.Chainring, chosenGear.Cog, chosenGear.Speed, chosenGear.Cadence, chosenGear.PowerLoss)

	return chosenGear
}



func calculatePowerLoss(gear Gear, power float64, chainringPowerLoss []ChainringPowerLoss, ftpZones []FTPZone) float64 {
	// Calculate the base power loss from the chainring power loss for that gear
	
	lossData := chainringPowerLoss[gear.GearNumber]
	basePowerLoss := lossData.Power * lossData.BadGearFactor
	//fmt.Printf("Gear: %d, Base Power Loss: %f\n", gear.GearNumber, basePowerLoss)

	// Determine which FTP zone the power falls in
	var coefficient float64
	for _, zone := range ftpZones {
		if power >= zone.Min && power < zone.Max {
			coefficient = zone.Coefficient
			break
		}
	}
//	fmt.Printf("Power: %f, Coefficient: %f\n", power, coefficient)

	// Calculate the total power loss with the FTP zone coefficient
	totalPowerLoss := basePowerLoss * coefficient
//	fmt.Printf("Total Power Loss: %f\n", totalPowerLoss)
	if math.IsNaN(power) {
		return 0
	}
	// Print gear details and calculated power loss
	//fmt.Printf("Gear Details: Gear Number - %d, Chainring - %d, Cog - %d, Speed - %f, Cadence - %f\n", gear.GearNumber, gear.Chainring, gear.Cog, gear.Speed, gear.Cadence)

	return totalPowerLoss
}


// Function to calculate the most optimal chainring
func calculateOptimalChainring(dataPoints []DataPoint, possibleChainrings []int, cassette []int, tyreCircumference float64, chainringPowerLoss []ChainringPowerLoss, ftpZones []FTPZone, goodGears []int, weight float64) int {
	var optimalChainring int
	var minWeightedLoss float64 = math.MaxFloat64
	
	// Loop over each possible chainring
	for _, chainring := range possibleChainrings {
		var totalPowerLoss float64
		var goodGearCount int

		// Loop over each data point
		for _, point := range dataPoints {
			if point.Power == 0 || math.IsNaN(point.Power) {
				continue
			}
			fmt.Println(point.Speed, "this is speed")
			gear := calculateGear(point.Speed, float64(point.Cad), chainring, cassette, tyreCircumference, chainringPowerLoss, ftpZones)
			//powerLoss := calculatePowerLoss(gear, point.Power, chainringPowerLoss, ftpZones)
			
			// Add the power loss for this data point to the total power loss
			totalPowerLoss += gear.PowerLoss
			
			// Check if the gear is within the 'good' range
			for _, goodGear := range goodGears {
				if gear.Cog == goodGear {
					goodGearCount++
					break
				}
			}
		}
		
		// Calculate the weighted power loss
		percentInGoodGears := float64(goodGearCount) / float64(len(dataPoints))
		// Print chainring and total power loss before calculating weighted loss
		//fmt.Printf("Chainring: %d, Total Power Loss: %f\n", chainring, totalPowerLoss)
		weightedLoss := totalPowerLoss - weight*percentInGoodGears
		// Print chainring and weighted loss after calculating weighted loss
	//	fmt.Printf("Chainring: %d, Weighted Loss: %f\n", chainring, weightedLoss)
	//	fmt.Printf("Chainring: %d, Total Power Loss: %f, Weighted Loss: %f\n", chainring, totalPowerLoss, weightedLoss)

		
		// If this chainring has a smaller weighted loss, it becomes the new optimal chainring
		if weightedLoss < minWeightedLoss {
			optimalChainring = chainring
			minWeightedLoss = weightedLoss
		}
	}


	return optimalChainring
}

func main2() {
	// Define your parameters
	possibleChainrings := []int{30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50}
	cassette := []int{11, 12, 13, 14, 15, 16, 17, 19, 21, 24, 27, 30}
	tyreCircumference := 2171.0
	
	// Define power loss for each gear
	chainringPowerLoss := []ChainringPowerLoss{
		{1, 9.0, 1.5},
		{2, 8.2, 1.35},
		{3, 7.5, 1.2},
		{4, 7.0, 1.0},
		{5, 7.0, 1.0},
		{6, 7.0, 1.0},
		{7, 7.0, 1.0},
		{8, 7.0, 1.0},
		{9, 7.5, 1.0},
		{10, 8.0, 1.2},
		{11, 8.8, 1.35},
		{12, 9.0, 1.5},
	}
	
	// Define FTP zones
	ftpZones := []FTPZone{
		{"Active Recovery", 0, 174, 0}, 
		{"Endurance", 174, 241, 0.5}, 
		{"Tempo", 241, 286, 0.75},
		{"Sweet Spot", 279, 333, 1},
		{"VO2 max", 333, 381, 1.5},
		{"Anaerobic capacity", 381, 477, 2},
		{"Neuromuscular", 477, 2000, 3},
	}
	gpxFile := "test.gpx"
	dataPoints, err := parseGPX(gpxFile)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	// Print dataPoints after parsing GPX file
fmt.Printf("Data Points: %v\n", dataPoints)
	// Now dataPoints has all points with the data from the GPX file
	//fmt.Println(dataPoints)

	goodGears := []int{12, 13, 14, 15}
	weight := 1.0
	optimalChainring := calculateOptimalChainring(dataPoints, possibleChainrings, cassette, tyreCircumference, chainringPowerLoss, ftpZones, goodGears, weight)
	fmt.Printf("Optimal chainring: %d\n", optimalChainring)
	// fmt.Println(ftpZones)
	//fmt.Println(dataPoints)
}
