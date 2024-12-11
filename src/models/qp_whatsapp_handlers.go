package models

import (
	"sync"

	"github.com/nocodeleaks/quepasa/library"
	whatsapp "github.com/nocodeleaks/quepasa/whatsapp"
	log "github.com/sirupsen/logrus"
)

// Serviço que controla os servidores / bots individuais do whatsapp
type QPWhatsappHandlers struct {
	QpWhatsappMessages
	library.LogStruct // logging

	server *QpWhatsappServer

	syncRegister *sync.Mutex

	// Appended events handler
	aeh []QpWebhookHandlerInterface
}

// get default log entry, never nil
func (source *QPWhatsappHandlers) GetLogger() *log.Entry {
	if source.server != nil {
		return source.server.GetLogger()
	}

	return source.LogStruct.GetLogger()
}

func (source *QPWhatsappHandlers) HandleGroups() bool {
	global := whatsapp.Options

	var local whatsapp.WhatsappBoolean
	if source.server != nil {
		local = source.server.Groups
	}
	return global.HandleGroups(local)
}

func (source *QPWhatsappHandlers) HandleBroadcasts() bool {
	global := whatsapp.Options

	var local whatsapp.WhatsappBoolean
	if source.server != nil {
		local = source.server.Broadcasts
	}
	return global.HandleBroadcasts(local)
}

//#region EVENTS FROM WHATSAPP SERVICE

// Process messages received from whatsapp service
func (source *QPWhatsappHandlers) Message(msg *whatsapp.WhatsappMessage, from string) {

	// should skip groups ?
	if !source.HandleGroups() && msg.FromGroup() {
		return
	}

	// should skip broadcast ?
	if !source.HandleBroadcasts() && msg.FromBroadcast() {
		return
	}

	// messages sended with chat title
	if len(msg.Chat.Title) == 0 {
		msg.Chat.Title = source.server.GetChatTitle(msg.Chat.Id)
	}

	if len(msg.InReply) > 0 {
		cached, err := source.QpWhatsappMessages.GetById(msg.InReply)
		if err == nil {
			maxlength := ENV.SynopsisLength() - 4
			if uint64(len(cached.Text)) > maxlength {
				msg.Synopsis = cached.Text[0:maxlength] + " ..."
			} else {
				msg.Synopsis = cached.Text
			}
		}
	}

	logentry := source.GetLogger()
	logentry = logentry.WithField(LogFields.MessageId, msg.Id)
	logentry = logentry.WithField(LogFields.ChatId, msg.Chat.Id)
	logentry.Debugf("appending message to cache, from: %s", from)
	source.appendMsgToCache(msg, from)
}

// region STATUS AND RECEIPTS

// does not cache msg, only update status and webhook dispatch
func (source *QPWhatsappHandlers) Receipt(msg *whatsapp.WhatsappMessage) {
	// should implement a better method for that !!!!
	// should implement a better method for that !!!!
	// should implement a better method for that !!!!
	// should implement a better method for that !!!!
	// should implement a better method for that !!!!

	// triggering external publishers
	source.Trigger(msg)
}

//endregion

/*
<summary>

	Event on:
		* User Logged Out from whatsapp app
		* Maximum numbers of devices reached
		* Banned
		* Token Expired

</summary>
*/
func (source *QPWhatsappHandlers) LoggedOut(reason string) {

	// one step at a time
	if source.server != nil {

		msg := "logged out !"
		if len(reason) > 0 {
			msg += " reason: " + reason
		}

		logger := source.GetLogger()
		logger.Warn(msg)

		// marking unverified and wait for more analyses
		source.server.MarkVerified(false)
	}
}

/*
<summary>

	Event on:
		* When connected to whatsapp servers and authenticated

</summary>
*/
func (source *QPWhatsappHandlers) OnConnected() {

	// one step at a time
	if source.server != nil {

		// marking unverified and wait for more analyses
		err := source.server.MarkVerified(true)
		if err != nil {
			logger := source.server.GetLogger()
			logger.Errorf("error on mark verified after connected: %s", err.Error())
		}
	}
}

/*
<summary>

	Event on:
		* When connected to whatsapp servers and authenticated

</summary>
*/
func (source *QPWhatsappHandlers) OnDisconnected() {

}

//#endregion
//region MESSAGE CONTROL REGION HANDLE A LOCK

// Salva em cache e inicia gatilhos assíncronos
func (source *QPWhatsappHandlers) appendMsgToCache(msg *whatsapp.WhatsappMessage, from string) {

	// saving on local normalized cache, do not affect remote msgs
	valid := source.QpWhatsappMessages.Append(msg, from)

	// should cleanup old messages ?
	length := ENV.CacheLength()
	source.QpWhatsappMessages.CleanUp(length)

	// continue to external dispatchers
	if valid {
		source.Trigger(msg)
	}
}

func (source *QPWhatsappHandlers) GetById(id string) (*whatsapp.WhatsappMessage, error) {
	return source.QpWhatsappMessages.GetById(id)
}

// endregion
// region EVENT HANDLER TO INTERNAL USE, GENERALLY TO WEBHOOK

// sends the message throw external publishers
func (source *QPWhatsappHandlers) Trigger(payload *whatsapp.WhatsappMessage) {
	if source != nil {
		if source.server != nil {
			go SignalRHub.Dispatch(source.server.Token, payload)
		}

		for _, handler := range source.aeh {
			go handler.HandleWebHook(payload)
		}
	}
}

// Register an event handler that triggers on a new message received on cache
func (handler *QPWhatsappHandlers) Register(evt QpWebhookHandlerInterface) {
	handler.syncRegister.Lock() // await for avoid simultaneous calls

	if !handler.IsRegistered(evt) {
		handler.aeh = append(handler.aeh, evt)
	}

	handler.syncRegister.Unlock()
}

// Removes an specific event handler
func (handler *QPWhatsappHandlers) UnRegister(evt QpWebhookHandlerInterface) {
	handler.syncRegister.Lock() // await for avoid simultaneous calls

	newHandlers := []QpWebhookHandlerInterface{}
	for _, v := range handler.aeh {
		if v != evt {
			newHandlers = append(handler.aeh, evt)
		}
	}

	// updating
	handler.aeh = newHandlers

	handler.syncRegister.Unlock()
}

// Removes an specific event handler
func (handler *QPWhatsappHandlers) Clear() {
	handler.syncRegister.Lock() // await for avoid simultaneous calls

	// updating
	handler.aeh = nil

	handler.syncRegister.Unlock()
}

// Indicates that has any event handler registered
func (handler *QPWhatsappHandlers) IsAttached() bool {
	return len(handler.aeh) > 0
}

// Indicates that if an specific handler is registered
func (handler *QPWhatsappHandlers) IsRegistered(evt interface{}) bool {
	for _, v := range handler.aeh {
		if v == evt {
			return true
		}
	}

	return false
}

//endregion

func (source *QPWhatsappHandlers) IsInterfaceNil() bool {
	return nil == source
}
