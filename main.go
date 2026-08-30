package main

import "fmt"

type Notifier interface {
	Send(msg string) error
}

type EmailClient struct{ Address string }

func (e EmailClient) Send(msg string) error {
	if msg == "" {
		return fmt.Errorf("message cannot be empty")
	}
	fmt.Println("Sent:", msg)
	return nil
}

func main() {
	name := "Assignment" // short declaration; type inferred as string
	var client Notifier = EmailClient{Address: "user@example.com"}

	if err := client.Send(name); err != nil {
		fmt.Println("Error:", err)
	}
}
