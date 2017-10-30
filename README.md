# asterisk-zabbix-phone-escalation
Zabbix Alert Script/Asterisk Agi script for generating phone escallations/oncall rotations using https://github.com/mcilley/Asterisk-TTS & https://github.com/mcilley/zabbix-ack-event

#### Zabbix Action Configuration:
![Alt text](/images/action.png?raw=true "configuration of zabbix action" )

#### Generated email of oncall rotation (flag --emailsched=true):
Email generated with: https://github.com/matcornic/hermes
![Alt text](/images/email.png?raw=true "example email of oncall rotation" )


#### Excerpt from example extensions.conf for asterisk
```
[outgoing-notify]
exten => s,1,Answer()
 exten => s,n,NoOp()
 exten => s,n,WaitForSilence(1500)
 ;;set timeout 
 exten => s,n,Set(TIMEOUT(digit)=5)
 exten => s,n,agi(ttsGoogle.agi,"Hello ${NAME} You are being paged for the following alert:")
 exten => s,n,agi(ttsGoogle.agi," ${MESSAGE} ")
 ;;Wait for response:
 exten => s,n,Set(COUNT=1)
 exten => s,n,Set(RESPOND=0)
 exten => s,n(start),agi(ttsGoogle.agi,"Please Press 1 to Accept and Acknowlege this alert, Press 2 to Escalate this issue")
 exten => s,n,Set(COUNT=${INC(COUNT)})
 exten => s,n,WaitExten()
 
 ;;PLayback the name of the digit and ack alert in zabbix
 exten => 1,1,Set(RESPOND=1)
 exten => 1,n,agi(ackEvents.agi,"--name=${NAME}","--triggerId=${ALERTID}")
 exten => 1,n,agi(ttsGoogle.agi,"You have Pressed ${EXTEN} to Accept this issue, Goodbye!")
 exten => 1,n,Hangup()

 ;;Escallate to next person on
 exten => 2,1(escalate),Set(ESCALATION=${INC(ESCALATION)})
 exten => 2,n,Set(RESPOND=1)
 exten => 2,n,agi(alertEscalate.agi,"--message='${MESSAGE}'","--alertId=${ALERTID}","--contactsDb=/home/asterisk/asterisk_files/contacts.db","--outgoingDir=/var/spool/asterisk/outgoing/","--escalation=${ESCALATION}")
 exten => 2,n,agi(ttsGoogle.agi,"You have Pressed ${EXTEN} to Escallate this issue, The next available team member will now be paged, Goodbye!")
 exten => 2,n,Hangup()
 ```
