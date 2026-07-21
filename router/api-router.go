package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", controller.PostSetup)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/pricing", middleware.TryUserAuth(), controller.GetPricing)
		apiRouter.GET("/health/dashboard", controller.GetHealthDashboard)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), controller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), controller.GenerateOAuthCode)
		apiRouter.GET("/oauth/email/bind", middleware.CriticalRateLimit(), controller.EmailBind)
		// Non-standard OAuth (WeChat, Telegram) - keep original routes
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.GET("/oauth/wechat/bind", middleware.CriticalRateLimit(), controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), controller.TelegramLogin)
		apiRouter.GET("/oauth/telegram/bind", middleware.CriticalRateLimit(), controller.TelegramBind)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", controller.WaffoWebhook)

		// Universal secure verification routes
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.UniversalVerify)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/external_register", middleware.CriticalRateLimit(), controller.ExternalRegister)
			userRoute.POST("/external_subscription_quota", middleware.CriticalRateLimit(), controller.ExternalSubscriptionQuota)
			userRoute.POST("/external_topup", middleware.CriticalRateLimit(), controller.ExternalTopUp)
			userRoute.POST("/login", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Login)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), controller.Verify2FALogin)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.GET("/logout", controller.Logout)
			userRoute.POST("/epay/notify", controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/self/invite_codes", controller.GetSelfInviteCodes)
				selfRoute.GET("/self/invitees", controller.GetSelfInvitees)
				selfRoute.POST("/self/invitees/:id/enable_invitation", controller.EnableSelfInviteeInvitation)
				selfRoute.GET("/self/invite_agent_stats", controller.GetSelfInviteAgentStats)
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateAccessToken)
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), controller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), controller.RequestEpay)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.RequestCreemPay)
				selfRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPay)
				selfRoute.POST("/aff_transfer", controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", controller.Setup2FA)
				selfRoute.POST("/2fa/enable", controller.Enable2FA)
				selfRoute.POST("/2fa/disable", controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/:id/admin_menu_permissions", middleware.RootAuth(), controller.GetAdminMenuPermissions)
				adminRoute.PUT("/:id/admin_menu_permissions", middleware.RootAuth(), controller.UpdateAdminMenuPermissions)

				userAdminRoute := adminRoute.Group("/")
				userAdminRoute.Use(middleware.AdminMenuAuth("user"))
				{
					userAdminRoute.GET("/", controller.GetAllUsers)
					userAdminRoute.GET("/topup", controller.GetAllTopUps)
					userAdminRoute.POST("/topup/complete", controller.AdminCompleteTopUp)
					userAdminRoute.GET("/search", controller.SearchUsers)
					userAdminRoute.GET("/by_username", controller.GetUserByUsername)
					userAdminRoute.GET("/invite_consumption_breakdown", controller.GetInviteConsumptionBreakdown)
					userAdminRoute.GET("/:id/oauth/bindings", controller.GetUserOAuthBindingsByAdmin)
					userAdminRoute.DELETE("/:id/oauth/bindings/:provider_id", controller.UnbindCustomOAuthByAdmin)
					userAdminRoute.DELETE("/:id/bindings/:binding_type", controller.AdminClearUserBinding)
					userAdminRoute.GET("/:id/extra_usable_groups", controller.GetUserExtraUsableGroups)
					userAdminRoute.PUT("/:id/extra_usable_groups", controller.UpdateUserExtraUsableGroups)
					userAdminRoute.GET("/:id/aggregate_group_ratio_overrides", controller.GetUserAggregateGroupRatioOverrides)
					userAdminRoute.GET("/:id/aggregate_group_ratio_overrides/models", controller.GetUserAggregateGroupRatioOverrideModels)
					userAdminRoute.PUT("/:id/aggregate_group_ratio_overrides", controller.UpdateUserAggregateGroupRatioOverrides)
					userAdminRoute.GET("/:id/invite_codes", controller.GetUserInviteCodesByAdmin)
					userAdminRoute.PUT("/:id/invite_binding", controller.UpdateUserInviteBinding)
					userAdminRoute.DELETE("/:id/invite_binding", controller.DeleteUserInviteBinding)
					userAdminRoute.GET("/:id", controller.GetUser)
					userAdminRoute.POST("/", controller.CreateUser)
					userAdminRoute.POST("/manage", controller.ManageUser)
					userAdminRoute.PUT("/", controller.UpdateUser)
					userAdminRoute.DELETE("/:id", controller.DeleteUser)
					userAdminRoute.DELETE("/:id/reset_passkey", controller.AdminResetPasskey)

					// Admin 2FA routes
					userAdminRoute.GET("/2fa/stats", controller.Admin2FAStats)
					userAdminRoute.DELETE("/:id/2fa", controller.AdminDisable2FA)
				}
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCreemPay)
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		subscriptionAdminRoute.Use(middleware.AdminMenuAuth("subscription"))
		{
			subscriptionAdminRoute.GET("/plans", controller.AdminListSubscriptionPlans)
			subscriptionAdminRoute.POST("/plans", controller.AdminCreateSubscriptionPlan)
			subscriptionAdminRoute.PUT("/plans/:id", controller.AdminUpdateSubscriptionPlan)
			subscriptionAdminRoute.PATCH("/plans/:id", controller.AdminUpdateSubscriptionPlanStatus)
			subscriptionAdminRoute.POST("/bind", controller.AdminBindSubscription)

			// User subscription management (admin)
			subscriptionAdminRoute.GET("/users/:id/subscriptions", controller.AdminListUserSubscriptions)
			subscriptionAdminRoute.POST("/users/:id/subscriptions", controller.AdminCreateUserSubscription)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", controller.AdminInvalidateUserSubscription)
			subscriptionAdminRoute.DELETE("/user_subscriptions/:id", controller.AdminDeleteUserSubscription)
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/epay/return", controller.SubscriptionEpayReturn)
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.GET("/external_register_auth_code", controller.GetExternalRegisterAuthCode)
			optionRoute.POST("/external_register_auth_code", controller.GenerateExternalRegisterAuthCode)
			optionRoute.DELETE("/external_register_auth_code", controller.DeleteExternalRegisterAuthCode)
			optionRoute.DELETE("/external_register_auth_codes", controller.DeleteAllExternalRegisterAuthCodes)
			optionRoute.GET("/external_topup_auth_code", controller.GetExternalTopupAuthCode)
			optionRoute.POST("/external_topup_auth_code", controller.GenerateExternalTopupAuthCode)
			optionRoute.DELETE("/external_topup_auth_code", controller.DeleteExternalTopupAuthCode)
			optionRoute.DELETE("/external_topup_auth_codes", controller.DeleteAllExternalTopupAuthCodes)
			optionRoute.GET("/channel_affinity_cache", controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", controller.ResetModelRatio)
			optionRoute.POST("/migrate_console_setting", controller.MigrateConsoleSetting) // 用于迁移检测的旧键，下个版本会删除
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		requestDumpRoute := apiRouter.Group("/request_dump")
		requestDumpRoute.Use(middleware.AdminAuth())
		requestDumpRoute.Use(middleware.AdminMenuAuth("request_dump"))
		{
			requestDumpRoute.GET("/status", controller.GetRequestDumpStatus)
			requestDumpRoute.POST("/start", controller.StartRequestDump)
			requestDumpRoute.POST("/stop", controller.StopRequestDump)
			requestDumpRoute.GET("/events", controller.GetRequestDumpEvents)
			requestDumpRoute.POST("/clear", controller.ClearRequestDumpEvents)
			requestDumpRoute.GET("/error_snapshots/status", controller.GetErrorSnapshotStatus)
			requestDumpRoute.PUT("/error_snapshots/settings", controller.UpdateErrorSnapshotSettings)
			requestDumpRoute.GET("/error_snapshots", controller.GetErrorSnapshots)
			requestDumpRoute.GET("/error_snapshots/select_options", controller.GetErrorSnapshotSelectOptions)
			requestDumpRoute.DELETE("/error_snapshots", controller.DeleteAllErrorSnapshots)
			requestDumpRoute.POST("/error_snapshots/cleanup", controller.CleanupErrorSnapshots)
			requestDumpRoute.GET("/error_snapshots/:id", controller.GetErrorSnapshot)
			requestDumpRoute.GET("/error_snapshots/:id/download", controller.DownloadErrorSnapshot)
			requestDumpRoute.DELETE("/error_snapshots/:id", controller.DeleteErrorSnapshot)
		}
		violationRoute := apiRouter.Group("/violation")
		violationRoute.Use(middleware.AdminAuth())
		violationRoute.Use(middleware.AdminMenuAuth("violation"))
		{
			violationRoute.GET("/status", controller.GetViolationStatus)
			violationRoute.PUT("/setting", controller.UpdateViolationSetting)
			violationRoute.GET("/logs", controller.GetViolationLogs)
			violationRoute.DELETE("/logs", controller.DeleteViolationLogs)
		}
		channelRoute := apiRouter.Group("/channel")
		channelRoute.Use(middleware.AdminAuth())
		channelRoute.Use(middleware.AdminMenuAuth("channel"))
		{
			channelRoute.GET("/", controller.GetAllChannels)
			channelRoute.GET("/search", controller.SearchChannels)
			channelRoute.GET("/models", controller.ChannelListModels)
			channelRoute.GET("/models_enabled", controller.EnabledListModels)
			channelRoute.GET("/:id", controller.GetChannel)
			channelRoute.POST("/:id/key", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.SecureVerificationRequired(), controller.GetChannelKey)
			channelRoute.GET("/test", controller.TestAllChannels)
			channelRoute.GET("/test/:id", controller.TestChannel)
			channelRoute.GET("/update_balance", controller.UpdateAllChannelsBalance)
			channelRoute.GET("/update_balance/:id", controller.UpdateChannelBalance)
			channelRoute.POST("/", controller.AddChannel)
			channelRoute.PUT("/", controller.UpdateChannel)
			channelRoute.DELETE("/disabled", controller.DeleteDisabledChannel)
			channelRoute.POST("/tag/disabled", controller.DisableTagChannels)
			channelRoute.POST("/tag/enabled", controller.EnableTagChannels)
			channelRoute.PUT("/tag", controller.EditTagChannels)
			channelRoute.DELETE("/:id", controller.DeleteChannel)
			channelRoute.POST("/batch", controller.DeleteChannelBatch)
			channelRoute.POST("/fix", controller.FixChannelsAbilities)
			channelRoute.GET("/fetch_models/:id", controller.FetchUpstreamModels)
			channelRoute.POST("/fetch_models", controller.FetchModels)
			channelRoute.POST("/codex/oauth/start", controller.StartCodexOAuth)
			channelRoute.POST("/codex/oauth/complete", controller.CompleteCodexOAuth)
			channelRoute.POST("/:id/codex/oauth/start", controller.StartCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/oauth/complete", controller.CompleteCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/refresh", controller.RefreshCodexChannelCredential)
			channelRoute.GET("/:id/codex/usage", controller.GetCodexChannelUsage)
			channelRoute.POST("/ollama/pull", controller.OllamaPullModel)
			channelRoute.POST("/ollama/pull/stream", controller.OllamaPullModelStream)
			channelRoute.DELETE("/ollama/delete", controller.OllamaDeleteModel)
			channelRoute.GET("/ollama/version/:id", controller.OllamaVersion)
			channelRoute.POST("/batch/tag", controller.BatchSetChannelTag)
			channelRoute.GET("/tag/models", controller.GetTagModels)
			channelRoute.POST("/copy/:id", controller.CopyChannel)
			channelRoute.POST("/multi_key/manage", controller.ManageMultiKeys)
			channelRoute.POST("/upstream_updates/apply", controller.ApplyChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/apply_all", controller.ApplyAllChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect", controller.DetectChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect_all", controller.DetectAllChannelUpstreamModelUpdates)
		}
		aggregateGroupRoute := apiRouter.Group("/aggregate_group")
		aggregateGroupRoute.Use(middleware.AdminAuth())
		aggregateGroupRoute.Use(middleware.AdminMenuAuth("aggregate_group"))
		{
			aggregateGroupRoute.GET("/", controller.GetAggregateGroups)
			aggregateGroupRoute.GET("/models", controller.GetAggregateGroupTargetModels)
			aggregateGroupRoute.GET("/categories", controller.GetAggregateGroupCategories)
			aggregateGroupRoute.POST("/categories", controller.CreateAggregateGroupCategory)
			aggregateGroupRoute.PUT("/categories/order", controller.ReorderAggregateGroupCategories)
			aggregateGroupRoute.PUT("/categories/assign", controller.AssignAggregateGroupCategories)
			aggregateGroupRoute.PUT("/categories/:id", controller.UpdateAggregateGroupCategory)
			aggregateGroupRoute.DELETE("/categories/:id", controller.DeleteAggregateGroupCategory)
			aggregateGroupRoute.GET("/:id/runtime", controller.GetAggregateGroupRuntime)
			aggregateGroupRoute.GET("/:id", controller.GetAggregateGroup)
			aggregateGroupRoute.POST("/", controller.CreateAggregateGroup)
			aggregateGroupRoute.PUT("/", controller.UpdateAggregateGroup)
			aggregateGroupRoute.DELETE("/:id", controller.DeleteAggregateGroup)
		}
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		redemptionRoute.Use(middleware.AdminMenuAuth("redemption"))
		{
			redemptionRoute.GET("/", controller.GetAllRedemptions)
			redemptionRoute.GET("/search", controller.SearchRedemptions)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", controller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}
		inviteCodeRoute := apiRouter.Group("/invite_code")
		inviteCodeRoute.Use(middleware.AdminAuth())
		inviteCodeRoute.Use(middleware.AdminMenuAuth("invite_code"))
		{
			inviteCodeRoute.GET("/", controller.GetAllInviteCodes)
			inviteCodeRoute.GET("/search", controller.SearchInviteCodes)
			inviteCodeRoute.GET("/consumption", controller.GetInviteConsumptionStats)
			inviteCodeRoute.GET("/consumption/user", controller.GetInviteConsumptionUserDetail)
			inviteCodeRoute.GET("/:id", controller.GetInviteCode)
			inviteCodeRoute.POST("/", controller.AddInviteCode)
			inviteCodeRoute.PUT("/", controller.UpdateInviteCode)
			inviteCodeRoute.DELETE("/:id", controller.DeleteInviteCode)
		}
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.GetAllLogs)
		logRoute.GET("/dashboard", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.GetLogsDashboard)
		logRoute.DELETE("/", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.GetLogsStat)
		logRoute.GET("/usage_stats", middleware.AdminAuth(), middleware.AdminMenuAuth("usage_stats"), controller.GetUsageStats)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/search", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.GetAllQuotaDates)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		prefillGroupRoute.Use(middleware.AdminMenuAuth("models"))
		{
			prefillGroupRoute.GET("/", controller.GetPrefillGroups)
			prefillGroupRoute.POST("/", controller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", controller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", controller.DeletePrefillGroup)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), middleware.AdminMenuAuth("log_dashboard"), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.POST("/callback/external-image/batch", controller.ImageTaskCallbackBatch)
			taskRoute.POST("/callback/external-image/:task_id", controller.ImageTaskCallback)
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			taskRoute.GET("/async/stats", middleware.AdminAuth(), middleware.AdminMenuAuth("async_task"), controller.GetAsyncTaskStats)
			taskRoute.GET("/async/image-handle/config", middleware.AdminAuth(), middleware.AdminMenuAuth("async_task"), controller.GetImageHandleConfig)
			taskRoute.PUT("/async/image-handle/config", middleware.AdminAuth(), middleware.AdminMenuAuth("async_task"), controller.UpdateImageHandleConfig)
			taskRoute.GET("/", middleware.AdminAuth(), middleware.AdminMenuAuth("async_task"), controller.GetAllTask)
			taskRoute.PUT("/:task_id/block", middleware.AdminAuth(), middleware.AdminMenuAuth("async_task"), controller.UpdateTaskBlockStatus)
		}

		internalImageLeaseRoute := apiRouter.Group("/internal/image/credential-leases")
		{
			internalImageLeaseRoute.POST("/:lease_id/resolve", controller.ResolveImageCredentialLease)
		}

		assetRoute := apiRouter.Group("/assets")
		{
			assetRoute.GET("/keys", middleware.UserAuth(), controller.GetUserAssetKeys)
			assetRoute.POST("/keys", middleware.UserAuth(), controller.CreateUserAssetKey)
			assetRoute.PUT("/keys/:id/status", middleware.UserAuth(), controller.UpdateUserAssetKeyStatus)
			assetRoute.DELETE("/keys/:id", middleware.UserAuth(), controller.DeleteUserAssetKey)
			assetRoute.GET("/self", middleware.TokenOrUserAuth(), controller.GetUserAssets)
			assetRoute.GET("/self/export", middleware.TokenOrUserAuth(), controller.ExportUserAssets)
			assetRoute.POST("/self/batch/urls", middleware.TokenOrUserAuth(), controller.GetUserAssetBatchURLs)
			assetRoute.GET("/self/:asset_id", middleware.TokenOrUserAuth(), controller.GetUserAsset)
			assetRoute.GET("/", middleware.AdminAuth(), middleware.AdminMenuAuth("assets"), controller.GetAllAssets)
			assetRoute.GET("/export", middleware.AdminAuth(), middleware.AdminMenuAuth("assets"), controller.ExportAssets)
			assetRoute.POST("/batch/urls", middleware.AdminAuth(), middleware.AdminMenuAuth("assets"), controller.GetAssetBatchURLs)
			assetRoute.GET("/:asset_id", middleware.AdminAuth(), middleware.AdminMenuAuth("assets"), controller.GetAsset)
			assetRoute.PUT("/:asset_id/block", middleware.AdminAuth(), middleware.AdminMenuAuth("assets"), controller.UpdateAssetBlockStatus)
		}

		webhookRoute := apiRouter.Group("/webhook")
		webhookRoute.Use(middleware.UserAuth())
		{
			webhookRoute.GET("", controller.GetAccountWebhook)
			webhookRoute.PUT("", controller.PutAccountWebhook)
			webhookRoute.DELETE("", controller.DeleteAccountWebhook)
			webhookRoute.POST("/test", controller.TestAccountWebhook)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		vendorRoute.Use(middleware.AdminMenuAuth("models"))
		{
			vendorRoute.GET("/", controller.GetAllVendors)
			vendorRoute.GET("/search", controller.SearchVendors)
			vendorRoute.GET("/:id", controller.GetVendorMeta)
			vendorRoute.POST("/", controller.CreateVendorMeta)
			vendorRoute.PUT("/", controller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", controller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		modelsRoute.Use(middleware.AdminMenuAuth("models"))
		{
			modelsRoute.GET("/sync_upstream/preview", controller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", controller.SyncUpstreamModels)
			modelsRoute.GET("/missing", controller.GetMissingModels)
			modelsRoute.GET("/", controller.GetAllModelsMeta)
			modelsRoute.GET("/search", controller.SearchModelsMeta)
			modelsRoute.GET("/:id", controller.GetModelMeta)
			modelsRoute.POST("/", controller.CreateModelMeta)
			modelsRoute.PUT("/", controller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", controller.DeleteModelMeta)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		deploymentsRoute.Use(middleware.AdminMenuAuth("deployment"))
		{
			deploymentsRoute.GET("/settings", controller.GetModelDeploymentSettings)
			deploymentsRoute.POST("/settings/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/", controller.GetAllDeployments)
			deploymentsRoute.GET("/search", controller.SearchDeployments)
			deploymentsRoute.POST("/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/hardware-types", controller.GetHardwareTypes)
			deploymentsRoute.GET("/locations", controller.GetLocations)
			deploymentsRoute.GET("/available-replicas", controller.GetAvailableReplicas)
			deploymentsRoute.POST("/price-estimation", controller.GetPriceEstimation)
			deploymentsRoute.GET("/check-name", controller.CheckClusterNameAvailability)
			deploymentsRoute.POST("/", controller.CreateDeployment)

			deploymentsRoute.GET("/:id", controller.GetDeployment)
			deploymentsRoute.GET("/:id/logs", controller.GetDeploymentLogs)
			deploymentsRoute.GET("/:id/containers", controller.ListDeploymentContainers)
			deploymentsRoute.GET("/:id/containers/:container_id", controller.GetContainerDetails)
			deploymentsRoute.PUT("/:id", controller.UpdateDeployment)
			deploymentsRoute.PUT("/:id/name", controller.UpdateDeploymentName)
			deploymentsRoute.POST("/:id/extend", controller.ExtendDeployment)
			deploymentsRoute.DELETE("/:id", controller.DeleteDeployment)
		}
	}
}
