all:
	go run main.go github.com

check: 
	gofmt -w -s .
	staticcheck

clean:

examples:
	go run main.go github.com
	echo "https://paepcke.de" | go run main.go 
