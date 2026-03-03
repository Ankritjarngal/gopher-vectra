run:
	go run main.go

test-brute:
	python3 scripts/testing_brute_meathod.py

clean:
	rm -f *.wal *.index *.db
	@echo "Database wiped."

status:
	curl http://localhost:8080/status