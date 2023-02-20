package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var coll *mongo.Collection

func main() {
	godotenv.Load()
	uri := os.Getenv("MONGODB_URI")

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll = client.Database("PetBook").Collection("Pets")

	r := gin.Default()
	r.GET("/pets", read)
	r.POST("/pets", create)
	r.PATCH("/pets", update)
	r.DELETE("/pets", delete)
	r.Run()
}

func read(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)

	// convert json query from body directly to bson, allows client to specify stuff like $gt
	var filter bson.M
	bson.UnmarshalExtJSON(body, true, &filter)

	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var results []Pet
	err = cursor.All(context.TODO(), &results)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// automatically marshalls pets to json
	c.JSON(http.StatusOK, results)
}

func create(c *gin.Context) {
	// this time unmarshal to a pet object since it's not really a query
	var p Pet
	body, _ := io.ReadAll(c.Request.Body)
	json.Unmarshal(body, &p)

	// now check that all required fields are filled out
	check := verify(p)
	if check != "" {
		c.String(http.StatusBadRequest, "Missing or invalid field: "+check)
		return
	}

	// provide default values for all non-required fields
	if p.Species == "dog" && p.Breed == "" {
		p.Breed = "unknown"
	}

	query, _ := bson.Marshal(p)

	res, err := coll.InsertOne(context.TODO(), query)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// read the newly created element so we can return it to the client
	filter := bson.D{{"_id", res.InsertedID}}
	coll.FindOne(context.TODO(), filter).Decode(&p)

	c.JSON(http.StatusOK, p)
}

func verify(p Pet) string {
	if p.Name == "" {
		return "name"
	}
	if p.Dob.IsZero() {
		return "dob"
	}
	if p.OwnerName == "" {
		return "owner_name"
	}
	if p.Species == "" {
		return "species"
	}
	if p.Height <= 0 {
		return "height"
	}
	if p.Weight <= 0 {
		return "weight"
	}
	if p.FavoriteToy == "" {
		return "favorite_toy"
	}
	return ""
}

func update(c *gin.Context) {
	// straight to bson
	var b []bson.M
	body, _ := io.ReadAll(c.Request.Body)
	bson.UnmarshalExtJSON(body, true, &b)

	if len(b) != 2 {
		c.String(http.StatusBadRequest, "Invalid request.")
		return
	}

	_, err := coll.UpdateMany(context.TODO(), b[0], b[1])
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// read all the records that were updated so we can return them to client
	cursor, err := coll.Find(context.TODO(), b[0])
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var results []Pet
	err = cursor.All(context.TODO(), &results)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, results)
}

func delete(c *gin.Context) {
	var filter bson.M
	body, _ := io.ReadAll(c.Request.Body)
	bson.UnmarshalExtJSON(body, true, &filter)

	res, err := coll.DeleteMany(context.TODO(), filter)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// return how many records were deleted
	c.JSON(http.StatusOK, res)
}

type Pet struct {
	Id          string    `json:"_id,omitempty" bson:"_id,omitempty"`
	Name        string    `json:"name,omitempty" bson:",omitempty"`
	Dob         time.Time `json:"dob,omitempty" bson:",omitempty"`
	OwnerName   string    `json:"owner_name,omitempty" bson:"owner_name,omitempty"`
	Species     string    `json:"species,omitempty" bson:",omitempty"`
	Height      int32     `json:"height,omitempty" bson:",omitempty"`
	Weight      int32     `json:"weight,omitempty" bson:",omitempty"`
	FavoriteToy string    `json:"favorite_toy,omitempty" bson:"favorite_toy,omitempty"`
	Breed       string    `json:"breed,omitempty" bson:",omitempty"`
}
