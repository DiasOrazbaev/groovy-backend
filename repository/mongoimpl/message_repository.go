package mongoimpl

import (
	"context"
	"github.com/DiasOrazbaev/groovy/model"
	"github.com/DiasOrazbaev/groovy/model/apperrors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

type MessageRepository struct {
	db         *mongo.Database
	collection string
}

func NewMessageRepository(db *mongo.Database, collection string) *MessageRepository {
	return &MessageRepository{db: db, collection: collection}
}

type messageQuery struct {
	ID            primitive.ObjectID `bson:"_id"`
	Text          string             `bson:"text"`
	CreatedAt     time.Time          `bson:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at"`
	FileType      string             `bson:"file_type"`
	Url           string             `bson:"url"`
	Filename      string             `bson:"filename"`
	AttachmentId  string             `bson:"attachment_id"`
	UserId        string             `bson:"user_id"`
	UserCreatedAt time.Time          `bson:"user_created_at"`
	UserUpdatedAt time.Time          `bson:"user_updated_at"`
	Username      string             `bson:"username"`
	Image         string             `bson:"image"`
	IsOnline      bool               `bson:"is_online"`
	Nickname      string             `bson:"nickname"`
	Color         string             `bson:"color"`
	IsFriend      bool               `bson:"is_friend"`
}

func (m *MessageRepository) GetMessages(userId string, channel *model.Channel, cursor string) (*[]model.MessageResponse, error) {
	// create a pipeline
	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "user_id",
				"foreignField": "id",
				"as":           "users",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "attachments",
				"localField":   "id",
				"foreignField": "message_id",
				"as":           "attachments",
			},
		},
		{
			"$match": bson.M{
				"channel_id": channel.ID,
			},
		},
		{
			"$sort": bson.M{
				"created_at": -1,
			},
		},
		{
			"$limit": 35,
		},
		{
			"$project": bson.M{
				"id":                  1,
				"text":                1,
				"created_at":          1,
				"updated_at":          1,
				"attachment.fileType": 1,
				"attachment.url":      1,
				"attachment.filename": 1,
				"attachment_id":       "$attachments.id",
				"user_id":             "$users.id",
				"user_created_at":     "$users.created_at",
				"user_updated_at":     "$users.updated_at",
				"username":            "$users.username",
				"image":               "$users.image",
				"is_online":           "$users.is_online",
				"is_friend": bson.M{
					"$cond": bson.A{
						bson.M{
							"$in": bson.A{
								userId,
								"$users.friends.friend_id",
							},
						},
						true,
						false,
					},
				},
			},
		},
	}

	// create a context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// execute the pipeline
	cur, err := m.db.Aggregate(ctx, pipeline)
	if err != nil {
		log.Println(err)
		return nil, apperrors.NewInternal()
	}

	var result []model.Message
	if err := cur.All(ctx, &result); err != nil {
		log.Println(err)
		return nil, apperrors.NewInternal()
	}

	res := make([]model.MessageResponse, len(result))

	for i := 0; i < len(result); i++ {
		res[i] = model.MessageResponse{
			Id:         result[i].ID,
			Text:       result[i].Text,
			CreatedAt:  result[i].CreatedAt,
			UpdatedAt:  result[i].UpdatedAt,
			Attachment: result[i].Attachment,
			User:       model.MemberResponse{},
		}
	}

	return &res, nil
}

// CreateMessage inserts the message in the DB
func (m *MessageRepository) CreateMessage(message *model.Message) (*model.Message, error) {
	_, err := m.db.Collection(m.collection).InsertOne(context.TODO(), message)
	if err != nil {
		log.Printf("Could not create a message for user: %v. Reason: %v\n", message.UserId, err)
		return nil, apperrors.NewInternal()
	}

	return message, nil
}

// UpdateMessage updates the message in the DB
func (m *MessageRepository) UpdateMessage(message *model.Message) error {
	_, err := m.db.Collection(m.collection).UpdateOne(context.TODO(), bson.M{"_id": message.ID}, bson.M{"$set": message})
	if err != nil {
		log.Printf("Could not update message with id: %v. Reason: %v\n", message.ID, err)
		return apperrors.NewInternal()
	}
	return nil
}

// DeleteMessage removes the message from the DB
func (m *MessageRepository) DeleteMessage(message *model.Message) error {
	_, err := m.db.Collection(m.collection).DeleteOne(context.TODO(), bson.M{"_id": message.ID})
	if err != nil {
		log.Printf("Could not delete message with id: %v. Reason: %v\n", message.ID, err)
		return apperrors.NewInternal()
	}
	return nil
}

// GetById fetches the message for the given id
func (m *MessageRepository) GetById(messageId string) (*model.Message, error) {
	var message model.Message
	err := m.db.Collection(m.collection).FindOne(context.TODO(), bson.M{"id": messageId}).Decode(&message)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apperrors.NewNotFound("message", messageId)
		}
		return nil, apperrors.NewInternal()
	}
	return &message, nil
}
