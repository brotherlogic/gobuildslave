import os
import subprocess

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
        for i in range(len(lines)):
            os.popen('echo "LINE ['+`i`+']= ' + lines[i].strip() + '" >> out.txt').readlines()
        for line in os.popen('cat out.txt | mail -s "Crash Report ' + name + '" brotherlogic@gmail.com').readlines():
            pass
    for line in os.popen('echo "" > out.txt').readlines():
        pass
    for line in os.popen('killall ' + name).readlines():
        pass
    subprocess.Popen(['./' + name])
