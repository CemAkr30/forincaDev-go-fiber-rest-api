package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"net/http"
)

type ErrorResponse struct {
	Status              int32                 `json:"status"`
	ErrorDetailResponse []ErrorDetailResponse `json:"errorDetail"`
}

type ErrorDetailResponse struct {
	FieldName   string `json:"fieldName"`
	Description string `json:"description"`
}

type UserCreateRequest struct {
	// back code `` json
	// İlk harfi büyük olmalı public olması için, küçük olursa private olur
	FirstName string `json:"firstName" validate:"required,min=2"`
	LastName  string `json:"lastName" validate:"required"`
	Email     string `json:"email" validate:"required"`
	Password  string `json:"password" validate:"required,min=8,max=16"`
	Age       int32  `json:"age" validate:"required,acceptAge"`
}

type UserCreateResponse struct {
	UID       string `json:"uid" validate:"required"`
	FirstName string `json:"firstName" validate:"required,min=2"`
	LastName  string `json:"lastName" validate:"required"`
	Email     string `json:"email" validate:"required"`
	Age       int32  `json:"age" validate:"required"`
}

type UserResponseList struct {
	UserCreateResponse []UserCreateResponse `json:"data"`
	Count              int32                `json:"count"`
}

type CustomValidationError struct {
	HasError bool
	Field    string
	Tag      string
	Param    string
	Value    interface{}
}

var validate = validator.New()

func Validate(data interface{}) []CustomValidationError {
	var customValidationError []CustomValidationError
	if errors := validate.Struct(data); errors != nil {
		for _, fieldError := range errors.(validator.ValidationErrors) {
			var cve CustomValidationError
			cve.HasError = true
			cve.Field = fieldError.Field()
			cve.Param = fieldError.Param()
			cve.Tag = fieldError.Tag()
			cve.Value = fieldError.Value()

			customValidationError = append(customValidationError, cve)
		}
	}

	return customValidationError
}

func GenerateUUID() string {
	u := make([]byte, 16)
	_, err := rand.Read(u)
	if err != nil {
		return ""
	}

	u[8] = (u[8] | 0x80) & 0xBF // what does this do?
	u[6] = (u[6] | 0x40) & 0x4F // what does this do?

	return hex.EncodeToString(u)
}

var userCreateResponseList []UserCreateResponse

func main() {

	app := fiber.New()

	// validate

	validate.RegisterValidation("acceptAge", func(fl validator.FieldLevel) bool {
		return fl.Field().Int() >= 18
	})

	// Middleware (Bütün endpointleri cover eder)
	app.Use(func(ctx *fiber.Ctx) error {
		fmt.Printf("Hello client, you are call my %s%s And Method %s \n",
			ctx.BaseURL(),
			ctx.Request().RequestURI(),
			ctx.Request().Header.Method(),
		)

		return ctx.Next()
	})

	// Route öncesi middleware çalışacak fakat /user endpoint olanlar sadece bu middleware kullanabilir
	app.Use("/user", func(ctx *fiber.Ctx) error {
		correlationId := ctx.Get("X-CorrelationId")

		if correlationId == "" {
			return ctx.Status(http.StatusBadRequest).JSON("You have to send correlationId")
		}

		_, err := uuid.Parse(correlationId)

		if err != nil {
			return ctx.Status(http.StatusBadRequest).JSON("CorrelationId is must be guid")
		}

		ctx.Locals("correlationId", correlationId)

		return ctx.Next()
	})

	// Uygulama crush yediği down olduğu zaman bu middleware çalışır ve uygulamayı down etmez stabil ayakta kalır
	app.Use(recover.New())

	// API
	app.Get("/panic", func(ctx *fiber.Ctx) error {
		fmt.Println("Hello recover middleware test")
		panic("The app was crashing!!!")

		return ctx.Next()
	})

	app.Get("/", func(ctx *fiber.Ctx) error {
		fmt.Println("Hello first get endpoint")
		ctx.Status(200)
		return ctx.SendString("hello my first get endpoint")
	})

	app.Get("/user/:userId", func(ctx *fiber.Ctx) error {
		userIdParam := ctx.Params("userId")

		fmt.Printf("User id is %s", userIdParam)
		ctx.Status(200)
		return ctx.SendString("User id is " + userIdParam)
	})

	app.Post("/user", func(ctx *fiber.Ctx) error {

		var userCreateResponse UserCreateResponse

		fmt.Printf("hello my first post endpoint\n")
		var request UserCreateRequest

		err := ctx.BodyParser(&request)

		if err != nil {
			_ = fmt.Sprintf("There was an error while binding json -> Error %v ", err.Error())
			return err
		}

		if errors := Validate(request); errors != nil && errors[0].HasError {
			var errorResponse ErrorResponse
			var errorDetailResponseList []ErrorDetailResponse
			for _, validationError := range errors {
				var errorDetailResponse ErrorDetailResponse
				errorDetailResponse.FieldName = validationError.Field
				errorDetailResponse.Description = fmt.Sprintf("%s field has an error because of that tag %s", validationError.Field, validationError.Tag)

				errorDetailResponseList = append(errorDetailResponseList, errorDetailResponse)
			}
			errorResponse.Status = http.StatusBadRequest
			errorResponse.ErrorDetailResponse = errorDetailResponseList

			return ctx.Status(http.StatusBadRequest).JSON(errorResponse)
		}

		userCreateResponse.UID = GenerateUUID()
		userCreateResponse.FirstName = request.FirstName
		userCreateResponse.LastName = request.LastName
		userCreateResponse.Email = request.Email
		userCreateResponse.Age = request.Age

		userCreateResponseList = append(userCreateResponseList, userCreateResponse)

		responseMessage := fmt.Sprintf("%s User created successfully\n", request.FirstName)
		fmt.Printf(responseMessage)
		return ctx.Status(http.StatusOK).JSON(userCreateResponse)
	})

	app.Get("/user", func(ctx *fiber.Ctx) error {
		var userResponseList UserResponseList
		if userCreateResponseList == nil && len(userCreateResponseList) == 0 {
			return ctx.Status(http.StatusNotFound).JSON("There is no user")
		}

		userResponseList.UserCreateResponse = userCreateResponseList
		userResponseList.Count = int32(len(userCreateResponseList))

		return ctx.Status(http.StatusOK).JSON(userResponseList)
	})

	err := app.Listen(":3000")
	if err != nil {
		return
	}
}
