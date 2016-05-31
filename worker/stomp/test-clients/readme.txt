сlient1 sends messages to the queue /queue/ONE; subscribed to the /queue/TWO 
Client2 subscribed to the queue /queue/ONE; when he received message, he resends it to the queue /queue/TWO

	/queue/ONE		/queue/ONE						
|-------|======>|-------|======>|-------|-------|
|  cl1	|	|	|server	|	|  cl2	|
|	|	|	|	|	|	|
|-------|<======|-------|<======|-------|-------|
	/queue/TWO		/queue/TWO
