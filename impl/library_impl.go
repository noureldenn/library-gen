package impl

import (
	"library-gen/api"
	"library-gen/db"
	"library-gen/models"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LibraryServer struct{}

// 1. Register
func (s *LibraryServer) Register(c *gin.Context) {
	var input api.RegisterInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	member := models.MemberEntity{Name: input.Name, Email: input.Email, Password: string(hash)}

	if err := db.DB.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create member"})
		return
	}

	// Convert DB Entity to API Response
	id := int(member.ID)
	c.JSON(http.StatusOK, api.Member{Id: &id, Name: &member.Name, Email: &member.Email})
}

// 2. Login
func (s *LibraryServer) Login(c *gin.Context) {
	var input api.LoginInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Input"})
		return
	}

	var member models.MemberEntity
	if err := db.DB.Where("email = ?", input.Email).First(&member).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(member.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// JWT Generation
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": member.ID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenStr, _ := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	c.JSON(http.StatusOK, api.TokenResponse{Token: &tokenStr})
}

// 3. Get Books
func (s *LibraryServer) GetBooks(c *gin.Context) {
	var entities []models.BookEntity
	db.DB.Find(&entities)

	// Map DB entities to API response
	var response []api.Book
	for _, e := range entities {
		id := int(e.ID)
		response = append(response, api.Book{
			Id: &id, Title: &e.Title, Author: &e.Author, Stock: &e.Stock,
		})
	}
	c.JSON(http.StatusOK, response)
}

// 4. Create Book
func (s *LibraryServer) CreateBook(c *gin.Context) {
	var input api.Book
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
		return
	}

	entity := models.BookEntity{
		Title:  *input.Title,
		Author: *input.Author,
		Stock:  *input.Stock,
	}
	db.DB.Create(&entity)

	id := int(entity.ID)
	input.Id = &id // update ID in response
	c.JSON(http.StatusOK, input)
}

// 5. Borrow Book
func (s *LibraryServer) BorrowBook(c *gin.Context) {
	var input api.BorrowInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
		return
	}

	tx := db.DB.Begin()

	var book models.BookEntity
	// Note: We cast int to uint for GORM ID
	if err := tx.First(&book, input.BookId).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	if book.Stock < 1 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Out of stock"})
		return
	}

	borrow := models.BorrowEntity{
		MemberID: uint(input.MemberId),
		BookID:   uint(input.BookId),
	}

	if err := tx.Create(&borrow).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error"})
		return
	}

	book.Stock -= 1
	tx.Save(&book)
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"status": "borrowed", "borrow_id": borrow.ID})
}
