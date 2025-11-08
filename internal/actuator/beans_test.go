package actuator

import (
	"strconv"
	"testing"
)

func TestActuatorClientGetBeans(t *testing.T) {
	tests := []struct {
		name            string
		mockResponse    string
		mockStatus      int
		mockErr         error
		wantErr         bool
		wantContextsCnt int
	}{
		{
			name: "successful response with beans",
			mockResponse: `{
				"contexts": {
					"application": {
						"beans": {
							"myService": {
								"aliases": [],
								"scope": "singleton",
								"type": "com.example.MyService",
								"resource": "file [/app/classes/com/example/MyService.class]",
								"dependencies": ["myRepository", "myConfig"]
							},
							"myRepository": {
								"aliases": [],
								"scope": "singleton",
								"type": "com.example.MyRepository",
								"dependencies": []
							}
						}
					}
				}
			}`,
			mockStatus:      200,
			wantErr:         false,
			wantContextsCnt: 1,
		},
		{
			name: "multiple contexts",
			mockResponse: `{
				"contexts": {
					"application": {
						"beans": {
							"mainBean": {
								"aliases": [],
								"scope": "singleton",
								"type": "com.example.MainBean",
								"dependencies": []
							}
						}
					},
					"bootstrap": {
						"beans": {
							"configBean": {
								"aliases": ["config"],
								"scope": "singleton",
								"type": "com.example.ConfigBean",
								"dependencies": []
							}
						},
						"parent": "application"
					}
				}
			}`,
			mockStatus:      200,
			wantErr:         false,
			wantContextsCnt: 2,
		},
		{
			name:            "empty contexts",
			mockResponse:    `{"contexts": {}}`,
			mockStatus:      200,
			wantErr:         false,
			wantContextsCnt: 0,
		},
		{
			name:         "404 endpoint not found",
			mockResponse: ``,
			mockStatus:   404,
			wantErr:      true,
		},
		{
			name:         "500 internal server error",
			mockResponse: ``,
			mockStatus:   500,
			wantErr:      true,
		},
		{
			name:         "malformed JSON",
			mockResponse: `{"contexts": invalid}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					if path != "/beans" {
						t.Errorf("unexpected path: %s", path)
					}
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return &Response{
						Body:       []byte(tt.mockResponse),
						StatusCode: tt.mockStatus,
						Status:     strconv.Itoa(tt.mockStatus),
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetBeans()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetBeans() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Contexts) != tt.wantContextsCnt {
					t.Errorf("got %d contexts, want %d", len(result.Contexts), tt.wantContextsCnt)
				}
			}
		})
	}
}

func TestBeansResponseParsing(t *testing.T) {
	tests := []struct {
		name     string
		response string
		validate func(*testing.T, *BeansResponse)
	}{
		{
			name: "bean with all fields",
			response: `{
				"contexts": {
					"application": {
						"beans": {
							"testBean": {
								"aliases": ["alias1", "alias2"],
								"scope": "prototype",
								"type": "com.example.TestBean",
								"resource": "file [/app/TestBean.class]",
								"dependencies": ["dep1", "dep2", "dep3"]
							}
						}
					}
				}
			}`,
			validate: func(t *testing.T, resp *BeansResponse) {
				ctx, ok := resp.Contexts["application"]
				if !ok {
					t.Fatal("expected application context")
				}
				bean, ok := ctx.Beans["testBean"]
				if !ok {
					t.Fatal("expected testBean")
				}
				if len(bean.Aliases) != 2 {
					t.Errorf("expected 2 aliases, got %d", len(bean.Aliases))
				}
				if bean.Scope != "prototype" {
					t.Errorf("expected scope 'prototype', got '%s'", bean.Scope)
				}
				if bean.Type != "com.example.TestBean" {
					t.Errorf("expected type 'com.example.TestBean', got '%s'", bean.Type)
				}
				if len(bean.Dependencies) != 3 {
					t.Errorf("expected 3 dependencies, got %d", len(bean.Dependencies))
				}
			},
		},
		{
			name: "bean with empty arrays",
			response: `{
				"contexts": {
					"application": {
						"beans": {
							"simpleBean": {
								"aliases": [],
								"scope": "singleton",
								"type": "com.example.SimpleBean",
								"dependencies": []
							}
						}
					}
				}
			}`,
			validate: func(t *testing.T, resp *BeansResponse) {
				ctx := resp.Contexts["application"]
				bean := ctx.Beans["simpleBean"]
				if len(bean.Aliases) != 0 {
					t.Errorf("expected 0 aliases, got %d", len(bean.Aliases))
				}
				if len(bean.Dependencies) != 0 {
					t.Errorf("expected 0 dependencies, got %d", len(bean.Dependencies))
				}
			},
		},
		{
			name: "context with parent",
			response: `{
				"contexts": {
					"child": {
						"beans": {},
						"parent": "application"
					}
				}
			}`,
			validate: func(t *testing.T, resp *BeansResponse) {
				ctx := resp.Contexts["child"]
				if ctx.Parent != "application" {
					t.Errorf("expected parent 'application', got '%s'", ctx.Parent)
				}
			},
		},
		{
			name: "many beans in context",
			response: `{
				"contexts": {
					"application": {
						"beans": {
							"bean1": {"aliases": [], "scope": "singleton", "type": "Type1", "dependencies": []},
							"bean2": {"aliases": [], "scope": "singleton", "type": "Type2", "dependencies": []},
							"bean3": {"aliases": [], "scope": "singleton", "type": "Type3", "dependencies": []}
						}
					}
				}
			}`,
			validate: func(t *testing.T, resp *BeansResponse) {
				ctx := resp.Contexts["application"]
				if len(ctx.Beans) != 3 {
					t.Errorf("expected 3 beans, got %d", len(ctx.Beans))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					return &Response{
						Body:       []byte(tt.response),
						StatusCode: 200,
						Status:     "200",
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetBeans()

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.validate(t, result)
		})
	}
}
