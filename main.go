package main

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type URL struct {
	Value string `json:"url"`
}

type SlugURLPair struct {
	Slug     string    `bson:"slug"`
	Url      string    `bson:"url"`
	ExpireAt time.Time `bson:"expireAt"`
}

var SLUG_ALPHABET = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890_-")

// Generates a random string of length using the SLUG_ALPHABET
func generateRandomString(length int) string {
	s := make([]rune, length)
	for i := range s {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(SLUG_ALPHABET))))
		s[i] = SLUG_ALPHABET[randomIndex.Int64()]
	}
	return string(s)
}

// Generates and returns a 5-character long slug that is not in slugURLPairCollection
func generateUniqueSlug(slugURLPairCollection *mongo.Collection) string {
	s := generateRandomString(5)
	var result bson.M
	err := slugURLPairCollection.FindOne(context.TODO(), bson.D{{Key: "slug", Value: s}}).Decode(&result)
	for err == nil {
		err = slugURLPairCollection.FindOne(context.TODO(), bson.D{{Key: "slug", Value: s}}).Decode(&result)
	}
	return s
}

func createTTLIndex(slugURLPairCollection *mongo.Collection) error {
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "expireAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	_, err := slugURLPairCollection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		log.Panic(err)
	}
	return err
}

func isValidURL(str string) bool {
	resp, err := http.Get(str)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200
}

func assertProtocol(redirectUrl string) string {
	u, _ := url.Parse(redirectUrl)
	log.Default().Println(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		u.Scheme = "https"
	}
	return u.String()
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found.")
	}

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("You must set your 'MONGODB_URI' environmental variable. See\n\t https://www.mongodb.com/docs/drivers/go/current/usage-examples/#environment-variable")
	}

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	app := fiber.New()

	app.Post("/createSlugURLPair", func(c *fiber.Ctx) error {
		// get url from body
		url := new(URL)
		if err := c.BodyParser(url); err != nil {
			log.Default().Println(err)
			return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
				"err":     err.Error(),
				"message": "Body must contain a `url` field.",
			})
		}
		// ensure url leads with a protocol and that the url leads to a valid location
		url.Value = assertProtocol(url.Value)
		if !isValidURL(url.Value) {
			log.Default().Println("Error - Invalid URL:", url.Value)
			return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{"message": "Invalid URL"})
		}
		// get slugURLPairCollection
		slugURLPairCollection := client.Database("dlgfy").Collection("slug-url-pairs")
		// get unique slug
		uniqueSlug := generateUniqueSlug(slugURLPairCollection)
		// set expiration date to 5 minutes after creation date
		expireAt := time.Now().UTC().Add(time.Minute * 5)
		// set slugURLPair values
		slugURLPair := SlugURLPair{Slug: uniqueSlug, Url: url.Value, ExpireAt: expireAt}
		// insert slugURLPair into db
		result, err := slugURLPairCollection.InsertOne(context.TODO(), slugURLPair)
		if err != nil {
			log.Fatal(err)
		}
		// create TTL Index to remove expired SlugURLPairs
		if err = createTTLIndex(slugURLPairCollection); err != nil {
			log.Fatal(err)
		}
		log.Default().Println("Successfully inserted SlugURLPair", slugURLPair)
		return c.Status(fiber.StatusOK).JSON(&fiber.Map{
			"result":      result,
			"slugURLPair": slugURLPair,
		})
	})

	// redirects slug to url
	app.Get("/:slug", func(c *fiber.Ctx) error {
		slugParam := c.Params("slug")
		slugURLPairCollection := client.Database("dlgfy").Collection("slug-url-pairs")
		var result SlugURLPair
		err := slugURLPairCollection.FindOne(context.TODO(), bson.D{{Key: "slug", Value: slugParam}}).Decode(&result)
		if err != nil {
			log.Default().Println(err)
			return c.Status(fiber.StatusNotFound).SendString("404: Error - Unable to find redirection URL.")
		}

		log.Default().Println("Successful redirection to", result.Url)
		return c.Redirect(result.Url)
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(&fiber.Map{
			"message": "Hello Delongify!",
		})
	})

	log.Fatal(app.Listen(":3000"))
}