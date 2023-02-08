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

type GuildRepository struct {
	db         *mongo.Database
	collection string
}

func NewGuildRepository(db *mongo.Database, collection string) *GuildRepository {
	return &GuildRepository{db: db, collection: collection}
}

func (g *GuildRepository) FindUserByID(uid string) (*model.User, error) {
	var user model.User
	err := g.db.Collection(g.collection).FindOne(context.Background(), bson.M{"_id": uid}).Decode(&user)
	return &user, err
}

func (g *GuildRepository) FindByID(id string) (*model.Guild, error) {
	var guild model.Guild
	err := g.db.Collection(g.collection).FindOne(context.Background(), bson.M{"_id": id}).Decode(&guild)
	return &guild, err
}

// List returns all the given users guilds
func (g *GuildRepository) List(uid string) (*[]model.GuildResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	guilds := make([]model.GuildResponse, 0)

	pipe, err := g.db.Collection("guilds").Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"members.user_id": uid,
		}},
		{"$lookup": bson.M{
			"from":         "channels",
			"localField":   "id",
			"foreignField": "guild_id",
			"as":           "channels",
		}},
		{"$lookup": bson.M{
			"from":         "members",
			"localField":   "id",
			"foreignField": "guild_id",
			"as":           "members",
		}},
		{"$unwind": "$members"},
		{"$match": bson.M{
			"members.user_id": uid,
		}},
		{"$addFields": bson.M{
			"hasNotification": bson.M{
				"$gt": []any{
					bson.M{"$max": "$channels.last_activity"},
					"$members.last_seen",
				},
			},
			"default_channel_id": bson.M{
				"$arrayElemAt": []any{
					bson.M{"$sort": bson.M{"channels.created_at": 1}},
					"$channels.id",
				},
			},
		}},
		{"$project": bson.M{
			"_id":                0,
			"id":                 "$id",
			"name":               "$name",
			"owner_id":           "$owner_id",
			"icon":               "$icon",
			"created_at":         "$created_at",
			"updated_at":         "$updated_at",
			"hasNotification":    "$hasNotification",
			"default_channel_id": "$default_channel_id",
		}},
		{"$sort": bson.M{"created_at": 1}},
	})

	if err != nil {
		return nil, err
	}

	defer pipe.Close(ctx)

	for pipe.Next(ctx) {
		var guild model.GuildResponse
		if err := pipe.Decode(&guild); err != nil {
			return nil, err
		}
		guilds = append(guilds, guild)
	}

	if err := pipe.Err(); err != nil {
		return nil, err
	}

	return &guilds, nil
}

// GuildMembers returns all members of the given guild and
// whether they are the given user IDs friend
func (g *GuildRepository) GuildMembers(userId string, guildId string) (*[]model.MemberResponse, error) {
	ctx := context.TODO()

	var members []model.MemberResponse

	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "friends",
				"localField":   "id",
				"foreignField": "friend_id",
				"as":           "is_friend",
			},
		},
		{
			"$match": bson.M{
				"guild_id":          guildId,
				"is_friend.user_id": bson.M{"$eq": userId},
			},
		},
		{
			"$addFields": bson.M{
				"is_friend": bson.M{"$ne": []any{}},
			},
		},
		{
			"$project": bson.M{
				"_id":        0,
				"id":         "$id",
				"username":   "$username",
				"image":      "$image",
				"is_online":  "$is_online",
				"created_at": "$created_at",
				"updated_at": "$updated_at",
				"nickname":   "$nickname",
				"color":      "$color",
				"is_friend":  "$is_friend",
				"display_name": bson.M{
					"$ifNull": []any{"$nickname", "$username"},
				},
			},
		},
		{
			"$sort": bson.M{"display_name": 1},
		},
	}

	cur, err := g.db.Collection(g.collection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var member model.MemberResponse
		err := cur.Decode(&member)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return &members, nil

}

func (g *GuildRepository) VCMembers(guildId string) (*[]model.VCMemberResponse, error) {
	collection := g.db.Collection(g.collection)

	var members []model.VCMemberResponse
	cur, err := collection.Aggregate(context.TODO(), []bson.M{
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "user_id",
				"foreignField": "_id",
				"as":           "user",
			},
		},
		{
			"$unwind": "$user",
		},
		{
			"$lookup": bson.M{
				"from":         "members",
				"localField":   "user_id",
				"foreignField": "user_id",
				"as":           "member",
			},
		},
		{
			"$unwind": "$member",
		},
		{
			"$match": bson.M{
				"guild_id": bson.M{"$eq": guildId},
			},
		},
		{
			"$sort": bson.M{
				"$cond": []any{
					bson.M{"$ifNull": []any{"$member.nickname", "$user.username"}},
					1,
					"$user.username",
				},
			},
		},
		{
			"$project": bson.M{
				"_id":         0,
				"user_id":     "$user._id",
				"username":    "$user.username",
				"image":       "$user.image",
				"is_muted":    1,
				"is_deafened": 1,
				"nickname":    "$member.nickname",
			},
		},
	})

	for cur.Next(context.Background()) {
		var vc model.VCMemberResponse
		err := cur.Decode(&vc)
		if err != nil {
			return nil, err
		}
		members = append(members, vc)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return &members, err
}

func (g *GuildRepository) Create(guild *model.Guild) (*model.Guild, error) {
	guildColl := g.db.Collection(g.collection)
	guildDoc, err := guildColl.InsertOne(context.TODO(), guild)
	if err != nil {
		log.Printf("Could not create a guild for user: %v. Reason: %v\n", guild.OwnerId, err)
		return nil, apperrors.NewInternal()
	}

	guild.ID = guildDoc.InsertedID.(primitive.ObjectID).Hex()
	return guild, nil
}

func (g *GuildRepository) Save(guild *model.Guild) error {
	if _, err := g.db.Collection(g.collection).UpdateOne(context.TODO(), bson.M{"_id": guild.ID}, bson.M{"$set": guild}); err != nil {
		log.Printf("Could not update the guild with id: %v. Reason: %v\n", guild.ID, err)
		return apperrors.NewInternal()
	}

	return nil
}

func (g *GuildRepository) RemoveMember(userId string, guildId string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) Delete(guildId string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) UnbanMember(userId string, guildId string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) GetBanList(guildId string) (*[]model.BanResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) GetMemberSettings(userId string, guildId string) (*model.MemberSettings, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) UpdateMemberSettings(settings *model.MemberSettings, userId string, guildId string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) FindUsersByIds(ids []string, guildId string) (*[]model.User, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) GetMember(userId, guildId string) (*model.User, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) UpdateMemberLastSeen(userId, guildId string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) RemoveVCMember(userId, guildId string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) GetMemberIds(guildId string) (*[]string, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) UpdateVCMember(isMuted, isDeafened bool, userId, guildId string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GuildRepository) GetVCMember(userId, guildId string) (*model.VCMember, error) {
	//TODO implement me
	panic("implement me")
}
