package httpapi

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"

	"github.com/thomas666-beast/sindri/internal/admin"
	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/category"
	"github.com/thomas666-beast/sindri/internal/chat"
	"github.com/thomas666-beast/sindri/internal/crypto"
	"github.com/thomas666-beast/sindri/internal/currency"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/notification"
	"github.com/thomas666-beast/sindri/internal/order"
	"github.com/thomas666-beast/sindri/internal/product"
	"github.com/thomas666-beast/sindri/internal/profile"
	"github.com/thomas666-beast/sindri/internal/review"
	"github.com/thomas666-beast/sindri/internal/settings"
	"github.com/thomas666-beast/sindri/internal/user"
	"github.com/thomas666-beast/sindri/internal/wallet"
	"github.com/thomas666-beast/sindri/internal/withdrawal"
	"github.com/go-chi/httprate"
	"time"
)

type Deps struct {
	Pool           *pgxpool.Pool
	Auth           *auth.Manager
	ChatCipher     *crypto.Cipher
	CookieSecure   bool
	UploadsDir     string
	AllowedOrigins []string
}

func NewRouter(d Deps) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(LogErrors)
	r.Use(middleware.Recoverer)

	// Serve uploaded files
	fs := http.FileServer(http.Dir(d.UploadsDir))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", fs))

	queries := dbgen.New(d.Pool)
	userChecker := auth.NewDBUserChecker(queries)

	// services
	notifSvc := notification.NewService(queries)
	currencySvc := currency.NewService(queries)
	userSvc := user.NewService(queries)
	settingsSvc := settings.NewService(queries)
	walletSvc := wallet.NewService(d.Pool)
	productSvc := product.NewService(queries)
	orderSvc := order.NewService(d.Pool, settingsSvc, notifSvc, d.ChatCipher, d.UploadsDir)
	adminSvc := admin.NewService(queries, notifSvc)
	chatSvc := chat.NewService(queries, d.ChatCipher, notifSvc, d.UploadsDir)
	hub := chat.NewHub(chatSvc)
	categorySvc := category.NewService(queries)
	profileSvc := profile.NewService(queries)
	reviewSvc := review.NewService(queries)
	withdrawalSvc := withdrawal.NewService(d.Pool, notifSvc)

	// CONNECT NOTIFICATIONS TO WEBSOCKET
	notifSvc.OnCreate = func(n *notification.Notification) {
		hub.PushNotification(n.UserID, chat.NewOutboundNotification(n))
	}

	// health
	r.Get("/healthz", healthHandler(d.Pool))

	r.Route("/api/v1", func(r chi.Router) {

		// ---- public ----
		r.Route("/currencies", func(r chi.Router) {
			r.Get("/", listCurrenciesHandler(currencySvc))
		})

		r.Route("/categories", func(r chi.Router) {
			r.Get("/", listCategoriesHandler(categorySvc))
			r.Get("/slug/{slug}", getCategoryBySlugHandler(categorySvc))
			r.Get("/slug/{slug}/children", listChildrenHandler(categorySvc))
			r.Get("/{id}", getCategoryHandler(categorySvc))
		})

		r.Get("/products", listProductsHandler(productSvc))
		r.Get("/products/search", searchProductsHandler(productSvc))
		r.Get("/products/{id}", getProductHandler(productSvc))
		r.Get("/products/{id}/images", getProductImagesHandler(productSvc))

		r.Get("/users/search", searchUsersHandler(userSvc))
		r.Get("/users/{username}/profile", getPublicProfileHandler(profileSvc))

		r.Route("/reviews", func(r chi.Router) {
			r.Get("/seller/{id}", listSellerReviewsHandler(reviewSvc))
			r.Get("/seller/{id}/summary", sellerSummaryHandler(reviewSvc))
			r.Get("/product/{id}", listProductReviewsHandler(reviewSvc))
			r.Get("/product/{id}/summary", productSummaryHandler(reviewSvc))
		})

		r.Get("/users/{username}/stats", getGeekStatsHandler(profileSvc))

		r.Route("/auth", func(r chi.Router) {
			r.Use(httprate.LimitByIP(10, time.Minute))

			r.Post("/register", registerHandler(userSvc, d.Auth, d.CookieSecure))
			r.Post("/login", loginHandler(userSvc, d.Auth, d.CookieSecure))
			r.Post("/logout", logoutHandler(d.Auth, d.CookieSecure))
		})

		r.Get("/ws/chat", chatWSHandler(d.Auth, hub, d.AllowedOrigins))

		// ---- authenticated ----
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(d.Auth, userChecker))

			r.Get("/me", meHandler(userSvc))
			r.Get("/me/profile", getMyProfileHandler(profileSvc))
			r.Put("/me/profile", updateMyProfileHandler(profileSvc))
			r.Post("/me/avatar", uploadAvatarHandler(profileSvc, d.UploadsDir))

			r.Get("/dashboard/stats", getDashboardStatsHandler(orderSvc))

			r.Route("/wallet", func(r chi.Router) {
				r.Get("/", getWalletHandler(walletSvc))
				r.Post("/deposit", depositHandler(walletSvc, settingsSvc, userSvc))
				r.Get("/transactions", listTransactionsHandler(walletSvc))
			})

			r.Route("/withdrawals", func(r chi.Router) {
				r.Post("/", requestWithdrawalHandler(withdrawalSvc))
				r.Get("/", listMyWithdrawalsHandler(withdrawalSvc))
			})

			r.Post("/products", createProductHandler(productSvc))
			r.Delete("/products/{id}", deleteProductHandler(productSvc))
			r.Put("/products/{id}/category", setProductCategoryHandler(productSvc))
			r.Put("/products/{id}", updateProductHandler(productSvc))
			r.Post("/products/{id}/images", uploadProductImageHandler(productSvc, d.UploadsDir))

			r.Route("/orders", func(r chi.Router) {
				r.Post("/", createOrderHandler(orderSvc))
				r.Get("/", listOrdersHandler(orderSvc))
				r.Get("/{id}", getOrderHandler(orderSvc))
				r.Post("/{id}/confirm", confirmOrderHandler(orderSvc))
				r.Post("/{id}/dispute", disputeOrderHandler(orderSvc))
			})

			r.Post("/orders/{id}/deliverables", uploadDeliverableHandler(orderSvc))
			r.Post("/orders/{id}/deliverables/seal", sealDeliveryHandler(orderSvc))
			r.Get("/orders/{id}/deliverables", listDeliverablesHandler(orderSvc))
			r.Get("/orders/{id}/deliverables/{fileId}", downloadDeliverableHandler(orderSvc))

			r.Route("/invoices", func(r chi.Router) {
				r.Post("/", createInvoiceHandler(orderSvc, userSvc))
				r.Get("/", listInvoicesHandler(orderSvc))
				r.Get("/{id}", getInvoiceHandler(orderSvc))
				r.Post("/{id}/pay", payInvoiceHandler(orderSvc))
				r.Post("/{id}/cancel", cancelInvoiceHandler(orderSvc))
			})

			r.Post("/reviews", createReviewHandler(reviewSvc))

			r.Route("/notifications", func(r chi.Router) {
				r.Get("/", listNotificationsHandler(notifSvc))
				r.Get("/unread-count", unreadCountHandler(notifSvc))
				r.Post("/{id}/read", markNotificationReadHandler(notifSvc))
				r.Post("/read-all", markAllNotificationsReadHandler(notifSvc))
			})

			r.Get("/me/warnings", myWarningsHandler(adminSvc))
			r.Post("/me/warnings/{id}/acknowledge", acknowledgeWarningHandler(adminSvc))

			r.Route("/tickets", func(r chi.Router) {
				r.Post("/", createTicketHandler(adminSvc))
				r.Get("/", myTicketsHandler(adminSvc))
				r.Get("/{id}", getTicketHandler(adminSvc))
				r.Get("/{id}/messages", listTicketMessagesHandler(adminSvc))
				r.Post("/{id}/messages", addTicketMessageHandler(adminSvc))
			})

			r.Route("/chat", func(r chi.Router) {
				r.Get("/conversations", chatListMineHandler(chatSvc))
				r.Post("/conversations/dm", chatStartDMHandler(chatSvc))
				r.Post("/conversations/group", chatCreateGroupHandler(chatSvc))
				r.Get("/conversations/{id}", chatGetHandler(chatSvc))
				r.Get("/conversations/{id}/messages", chatListMessagesHandler(chatSvc))
				r.Post("/conversations/{id}/messages", chatSendMessageHandler(chatSvc))
				r.Post("/conversations/{id}/read", chatMarkReadHandler(chatSvc))
				r.Get("/unread", unreadMessagesHandler(chatSvc))
				r.Get("/attachments/{id}", chatDownloadAttachmentHandler(chatSvc))
				r.Post("/conversations/{id}/upload", chatUploadAttachmentHandler(chatSvc, hub))
			})
		})

		// ---- admin ----
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(d.Auth, userChecker))
			r.Use(auth.RequireAdmin)

			r.Route("/admin", func(r chi.Router) {
				r.Get("/stats", adminStatsHandler(adminSvc))

				r.Get("/settings", adminGetSettingsHandler(adminSvc))
				r.Put("/settings/{key}", adminUpdateSettingHandler(adminSvc))

				r.Get("/transactions", adminListTransactionsHandler(adminSvc))

				r.Get("/orders/disputed", adminListDisputedOrdersHandler(orderSvc))
				r.Get("/orders/{id}", adminGetOrderHandler(orderSvc))
				r.Post("/orders/{id}/refund", refundOrderHandler(orderSvc))
				r.Post("/orders/{id}/release", adminReleaseOrderHandler(orderSvc))

				r.Route("/users", func(r chi.Router) {
					r.Get("/", adminListUsersHandler(adminSvc))
					r.Get("/{id}", adminGetUserHandler(adminSvc))
					r.Post("/{id}/disable", adminDisableUserHandler(adminSvc))
					r.Post("/{id}/enable", adminEnableUserHandler(adminSvc))
					r.Post("/{id}/kyc", adminSetKYCHandler(adminSvc))
					r.Post("/{id}/admin", adminSetAdminHandler(adminSvc))
					r.Get("/{id}/warnings", adminListWarningsHandler(adminSvc))
					r.Post("/{id}/warnings", adminCreateWarningHandler(adminSvc))
				})

				r.Route("/tickets", func(r chi.Router) {
					r.Get("/", adminListTicketsHandler(adminSvc))
					r.Post("/{id}/status", adminSetTicketStatusHandler(adminSvc))
				})

				r.Route("/categories", func(r chi.Router) {
					r.Post("/", createCategoryHandler(categorySvc))
					r.Put("/{id}", updateCategoryHandler(categorySvc))
					r.Delete("/{id}", deleteCategoryHandler(categorySvc))
				})

				r.Route("/withdrawals", func(r chi.Router) {
					r.Get("/", adminListWithdrawalsHandler(withdrawalSvc))
					r.Post("/{id}/approve", adminApproveWithdrawalHandler(withdrawalSvc))
					r.Post("/{id}/reject", adminRejectWithdrawalHandler(withdrawalSvc))
				})
			})
		})
	})

	return r
}
