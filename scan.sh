#!/bin/bash

for i in $( seq $1 $2 )
do
	for j in {1..255}
	do
		res=$(curl http://172.21.$i.$j/cgi-bin/get_basic -s --connect-timeout 0.5)
		if [ $(echo "$res" | wc -c) != 1 ]
		then
			name="$(echo "${res:10}" | head -1)"
			echo "172.21.$i.$j: $name"
			echo "172.21.$i.$j: $name" >> devices.txt
		else
			echo "swing and a miss on 172.21.$i.$j"
		fi
	done
done
