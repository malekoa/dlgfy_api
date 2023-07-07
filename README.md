# Delongify - URL Shortener

Delongify is a URL shortener application built with Go Fiber and MongoDB. It allows you to create short and unique slugs for long URLs, which can then be used to redirect to the original URLs.

## Prerequisites

Before running the application, make sure you have the following set up:

1. Go programming language installed
2. MongoDB database set up with a valid connection URI
3. Fiber and MongoDB Go packages installed (`github.com/gofiber/fiber/v2` and `go.mongodb.org/mongo-driver`)

## Setup

1. Clone the repository.
2. Create a .env file in the project directory (if not already present) and set the MONGODB_URI environment variable to your MongoDB connection URI.

## Usage

### Creating Slug-URL Pair

To create a slug-url pair, make a POST request to `/createSlugURLPair` endpoint. Include the following in the request body:

```json
{
  "url": "https://example.com"
}
```

Replace https://example.com with the long URL you want to shorten. The endpoint will generate a unique slug for the URL and store it in the MongoDB database. The response will contain the result of the insert operation and the generated slug-url pair.

### Redirecting to Original URL

To redirect to the original URL using the created slug, make a GET request to `/:slug` endpoint. Replace `:slug` with the generated slug for the URL. The application will look up the slug in the database and redirect to the corresponding original URL.

### Hello World

A default route is set up at the root endpoint / that returns a JSON response with a "Hello Delongify!" message.

### Additional Functionality

The code includes the following additional functionality:

1. Validating URL: The isValidURL function checks if a given URL is valid by performing an HTTP GET request and checking the response status code.
2. Protocol Handling: The assertProtocol function ensures that the URL starts with a protocol (http:// or https://). If the protocol is missing, it assumes https://.
3. Generating Random Slug: The generateRandomString function generates a random string using the SLUG_ALPHABET character set. It is used to create unique slugs for shortened URLs.
4. Handling Expiration: The createTTLIndex function creates a TTL (Time-To-Live) index on the expireAt field of the slug-url pair collection. It ensures that expired entries are automatically removed from the database.

## Running the Application

To run the application, execute the following command:

```shell
go run main.go
```

The application will start listening on port 3000. You can access it via `http://localhost:3000`.

Make sure your MongoDB database is running and accessible with the provided connection URI.
