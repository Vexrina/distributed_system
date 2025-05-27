# запуск с нуля
boot:
	docker-compose down -v
	docker volume prune -f
	docker-compose up -d --build

# SingleCast
sc_node0:
	curl -X POST http://localhost:8080/value -d '{"value":"test","type":"single_cast"}'

# MultiCast
mc_node0:
	curl -X POST http://localhost:8080/value -d '{"value":"test","type":"multicast"}'

# Broadcast	
bc_node0:
	curl -X POST http://localhost:8080/value -d '{"value":"test","type":"broadcast"}'

# Gossip
gossip_node0:
	curl -X POST http://localhost:8080/value -d '{"value":"test","type":"gossip"}'