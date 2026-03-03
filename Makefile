run:
	go run main.go

test-brute:
	python3 scripts/testing_brute_meathod.py

clean:
	rm -f *.wal *.index *.db
	@echo "Database wiped."

status:
	curl http://localhost:8080/status


bulk:
	python3 scripts/bulk_vector_upload.py
indexdel:
	rm -f *.index