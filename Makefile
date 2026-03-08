.PHONY: demo build clean

demo:
	go run main.go --root ./testserver --build "go build -o ./testserver/bin.exe ./testserver" --exec "./testserver/bin.exe"

build:
	go build -o hotreload.exe .

clean:
	del /Q hotreload.exe 2>nul || true
	del /Q testserver\bin.exe 2>nul || true
