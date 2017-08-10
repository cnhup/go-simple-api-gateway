package server

import (
	"github.com/labstack/echo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/Soontao/go-simple-api-gateway/enforcer"
	"github.com/casbin/casbin"
	"github.com/labstack/echo/middleware"
	"net/url"
	"github.com/labstack/gommon/log"
	"github.com/Soontao/go-simple-api-gateway/user"
)

type GatewayServer struct {
	*echo.Echo                            // web service
	*casbin.Enforcer                      // authorization service
	resourceHost        *url.URL          // be protected http resource
	authUserService     *user.UserService // user authenticate service
	DefaultRegisterRole string            // Default New User Role
}

// NewGatewayServer instance
func NewGatewayServer(connStr string, resourceHostStr string, defaultRole ...string) (s *GatewayServer) {

	resourceHost, err := url.Parse(resourceHostStr)

	if err != nil {
		log.Fatal(err)
	}
	s = &GatewayServer{
		Echo:            echo.New(),
		Enforcer:        enforcer.NewCasbinEnforcer(connStr),
		resourceHost:    resourceHost,
		authUserService: user.NewUserService(connStr),
	}

	if len(defaultRole) == 1 {
		s.DefaultRegisterRole = defaultRole[0]
	} else {
		s.DefaultRegisterRole = "basic_role"
	}

	s.Use(NewCoockieSession())
	s.mountAuthenticateEndpoints()
	s.mountAuthorizationEndPoints()
	s.mountReverseProxy()
	// hide echo banner
	s.Echo.HideBanner = true
	// load casbin policy from db
	s.Enforcer.LoadPolicy()
	return
}

func (s *GatewayServer) mountReverseProxy() {
	s.Group("/").Use(enforcer.Middleware(s.Enforcer), middleware.Proxy(&middleware.RoundRobinBalancer{
		Targets: []*middleware.ProxyTarget{
			&middleware.ProxyTarget{
				URL: s.resourceHost,
			},
		},
	}))
}

func (s *GatewayServer) mountAuthenticateEndpoints() {
	api := s.Group("/_/auth/api")
	api.Any("/auth", s.userAuth)
	api.Any("/updatepassword", s.userUpdate)
	api.Any("/register", s.userRegister)
}

func (s *GatewayServer) mountAuthorizationEndPoints() {
	api := s.Group("/_/gateway/api")
	policy := api.Group("/policy")
	policy.GET("/", s.getPolicies).Name = "Get All Policies"
	policy.GET("/group", s.getGroupPolicies).Name = "Get Group Policies"
	policy.GET("/authorities", s.getAllAuthorities)
	policy.GET("/methods", s.getAllActions)
	policy.POST("/enforce", s.enforceAuth).Name = "Find Some Authority"
	policy.PUT("/", s.addPolicy).Name = "Add Policy"
	policy.DELETE("/", s.removePolicy).Name = "Remove Authority"
	role := api.Group("/role")
	role.GET("/", s.getAllRoles)
	role.PUT("/", s.addRoleToUser).Name = "Add Role To User"
	role.DELETE("/", s.removeRoleFromUser).Name = "Remove Role From User"
	role.GET("/users", s.getRoleUsers).Name = "Get Users of a Role"
	user := api.Group("/user")
	user.GET("/", s.getAllUsers)
	user.GET("/role", s.getUserRoles).Name = "Get Roles of a User"
}
