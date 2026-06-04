package alert

// Evaluate checks whether the current price satisfies the alert condition.
// Returns true if the alert should fire.
//
// Example:
//
//	alert := Alert{Symbol: "BTCUSDT", Condition: Above, Price: 70000}
//	if Evaluate(alert, 71000) {
//	    // send notification
//	}
func Evaluate(a Alert, currentPrice float64) bool {
	switch a.Condition {
	case Above:
		return currentPrice >= a.Price
	case Below:
		return currentPrice <= a.Price
	default:
		return false
	}
}
