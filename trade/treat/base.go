package treat

import (
	"fmt"

	envConfig "github.com/oldfritter/goDCE/config"
	"github.com/oldfritter/goDCE/initializers"
	. "github.com/oldfritter/goDCE/models"
	"github.com/oldfritter/goDCE/utils"
	"github.com/streadway/amqp"
)

var (
	Assignments = make(map[int]Market)
)

func InitAssignments() {

	mainDB := utils.MainDbBegin()
	defer mainDB.DbRollback()
	var markets []Market
	mainDB.Where("trade_treat_node = ?", envConfig.CurrentEnv.Node).Find(&markets)
	for _, market := range markets {
		market.Running = Assignments[market.Id].Running
		if market.MatchingAble && !market.Running {
			Assignments[market.Id] = market
		} else if !market.MatchingAble {
			delete(Assignments, market.Id)
		}
	}
	mainDB.DbRollback()
	for id, assignment := range Assignments {
		if assignment.MatchingAble && !assignment.Running {
			go func(id int) {
				a := Assignments[id]
				subscribeMessageByQueue(&a, amqp.Table{})
			}(id)
			assignment.Running = true
			Assignments[id] = assignment
		}
	}
}

func subscribeMessageByQueue(assignment *Market, arguments amqp.Table) error {
	channel, err := initializers.RabbitMqConnect.Channel()
	if err != nil {
		fmt.Errorf("Channel: %s", err)
	}

	channel.ExchangeDeclare((*assignment).TradeTreatExchange(), "topic", (*assignment).Durable, false, false, false, nil)
	channel.QueueBind((*assignment).TradeTreatQueue(), (*assignment).Code, (*assignment).TradeTreatExchange(), false, nil)

	go func(id int) {
		a := Assignments[id]
		channel, err := initializers.RabbitMqConnect.Channel()
		if err != nil {
			fmt.Errorf("Channel: %s", err)
		}
		msgs, err := channel.Consume(
			a.TradeTreatQueue(),
			"",
			false,
			false,
			false,
			false,
			nil,
		)
		for d := range msgs {
			Treat(&d.Body)
			d.Ack(a.Ack)
		}
		return
	}(assignment.Id)

	return nil
}

func SubscribeReload() (err error) {
	channel, err := initializers.RabbitMqConnect.Channel()
	if err != nil {
		fmt.Errorf("Channel: %s", err)
		return
	}
	channel.ExchangeDeclare(initializers.AmqpGlobalConfig.Exchange["default"]["key"], "topic", true, false, false, false, nil)
	channel.QueueBind(initializers.AmqpGlobalConfig.Queue["trade"]["reload"], initializers.AmqpGlobalConfig.Queue["trade"]["reload"], initializers.AmqpGlobalConfig.Exchange["default"]["key"], false, nil)
	return
}
