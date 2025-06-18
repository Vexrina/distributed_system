# запуск с нуля
boot:
	docker-compose down -v
	docker volume prune -f
	docker-compose up -d --build

gen:
	cd generate-compose/
	go run generate-compose.go
	cd ../

# SingleCast
sc_node0:
	curl -X POST http://localhost:8080/value -d '{"value":"test","type":"single_cast"}'

# MultiCast
mc_node0:
	curl -X POST http://localhost:8080/value -d '{"value":"test","type":"multicast"}'

# Broadcast	
bc_node0:
	curl -X POST http://localhost:8083/value -d '{"value":"test","type":"broadcast"}'

# Gossip
gossip_node0:
	curl -X POST http://localhost:8080/value -d '{"value":"test123","type":"gossip"}'