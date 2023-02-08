package mongoimpl

import (
	"context"
	"errors"
	"github.com/DiasOrazbaev/groovy/model"
	"github.com/DiasOrazbaev/groovy/model/apperrors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepository struct {
	DB *mongo.Collection
	db *mongo.Database
}

func NewUserRepository(DB *mongo.Collection, db *mongo.Database) *UserRepository {
	return &UserRepository{DB: DB, db: db}
}

func (u *UserRepository) FindByID(id string) (*model.User, error) {
	user := &model.User{}
	if err := u.DB.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return user, apperrors.NewNotFound("uid", id)
		}
		return user, apperrors.NewInternal()
	}
	return user, nil
}
func (u *UserRepository) Create(user *model.User) (*model.User, error) {
	_, err := u.DB.InsertOne(context.Background(), user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindByEmail retrieves user row by email address
func (u *UserRepository) FindByEmail(email string) (*model.User, error) {
	user := &model.User{}
	err := u.DB.FindOne(context.TODO(), bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return user, apperrors.NewNotFound("email", email)
		}
		return user, apperrors.NewInternal()
	}

	return user, nil
}

func (u *UserRepository) Update(user *model.User) error {
	_, err := u.DB.UpdateOne(context.Background(), bson.M{"_id": user.ID}, bson.M{"$set": user})
	return err
}

func (u *UserRepository) GetFriendAndGuildIds(userId string) ([]string, error) {
	var ids []string
	guilds, _ := u.db.Collection("guilds").Find(context.Background(), bson.M{"members.user_id": userId})
	for guilds.Next(nil) {
		var guild struct{ ID string }
		if err := guilds.Decode(&guild); err != nil {
			return nil, err
		}
		ids = append(ids, guild.ID)
	}
	if err := guilds.Close(context.Background()); err != nil {
		return nil, err
	}

	friends, _ := u.db.Collection("users").Aggregate(context.Background(), []bson.M{
		{
			"$lookup": bson.M{
				"from":         "friends",
				"localField":   "id",
				"foreignField": "user_id",
				"as":           "friend",
			},
		},
		{
			"$match": bson.M{"id": userId},
		},
		{
			"$unwind": "$friend",
		},
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "friend.friend_id",
				"foreignField": "id",
				"as":           "friend",
			},
		},
		{
			"$unwind": "$friend",
		},
		{
			"$project": bson.M{"friend.id": 1},
		},
	})

	for friends.Next(nil) {
		var friend struct{ ID string }
		if err := friends.Decode(&friend); err != nil {
			return nil, err
		}
		ids = append(ids, friend.ID)
	}
	if err := friends.Close(context.Background()); err != nil {
		return nil, err
	}

	return ids, nil

}

func (u *UserRepository) GetRequestCount(userId string) (int64, error) {
	count, err := u.DB.CountDocuments(context.TODO(), bson.M{"receiver_id": userId})
	return count, err
}
