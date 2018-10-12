import os
import subprocess
import socket

name = "gobuildslave"

current_hash = ""
if os.path.isfile('hash'):
    lines = open('hash').readlines()
    if len(lines) > 0:
        current_hash = lines[0]
new_hash = os.popen('git rev-parse HEAD').readlines()[0]
open('hash','w').write(new_hash)
    
# Move the old version over
for line in os.popen('cp ' + name + ' old' + name).readlines():
    print line.strip()

# Rebuild
for line in os.popen('go build').readlines():
    print line.strip()

size_1 = os.path.getsize('./old' + name)
size_2 = os.path.getsize('./' + name)

lines = os.popen('ps -ef | grep ' + name).readlines()
running = False
for line in lines:
    if "./" + name in line:
        running = True
              
if size_1 != size_2 or new_hash != current_hash or not running:
    if not running:
        for line in os.popen('cat out.txt | mail -E -s "Crash Report ' + name + ' on $(hostname) " brotherlogic@gmail.com').readlines():
            pass
    for line in os.popen('echo "" > out.txt').readlines():
        pass
    for line in os.popen('killall ' + name).readlines():
        pass

    if socket.gethostname() == "discover":
        subprocess.Popen(['./' + name, '--builds=false'])
    else:
        subprocess.Popen(['./' + name])
