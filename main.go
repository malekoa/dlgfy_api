package main

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8000"
	} else {
		port = string(":") + port
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

	app.Use(cors.New())

	app.Use(limiter.New(limiter.Config{
		Max:               20,
		Expiration:        1 * time.Minute,
		LimiterMiddleware: limiter.SlidingWindow{},
		KeyGenerator: func(c *fiber.Ctx) string {
			// I have no idea if this is safe, but it works. I'm doing this
			// because Heroku uses a proxy in front of my server
			return strings.Join(c.IPs(), "")
		},
	}))

	app.Post("/createSlugURLPair", func(c *fiber.Ctx) error {
		// get url from body
		bodyUrl := new(URL)
		if err := c.BodyParser(bodyUrl); err != nil {
			log.Default().Println(err)
			return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
				"err":     err.Error(),
				"message": "Body must contain a `url` field.",
			})
		}
		// ensure url leads with a protocol and that the url leads to a valid location
		url, err := url.Parse(bodyUrl.Value)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
				"err":     err.Error(),
				"message": "Invalid URL.",
			})
		}
		// ensure url has a scheme, default to http
		if url.Scheme == "" {
			url.Scheme = "http"
		}
		// get slugURLPairCollection
		slugURLPairCollection := client.Database("dlgfy").Collection("slug-url-pairs")
		// get unique slug
		uniqueSlug := generateUniqueSlug(slugURLPairCollection)
		// set expiration date to 5 days after creation date
		expireAt := time.Now().UTC().Add(time.Hour * 24 * 5)
		// set slugURLPair values
		slugURLPair := SlugURLPair{Slug: uniqueSlug, Url: url.String(), ExpireAt: expireAt}
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

	log.Fatal(app.Listen(port))
}
