package actuator

import "testing"

func TestActuatorClientGetBeans(t *testing.T) {
	tests := []struct {
		name            string
		mockResponse    string
		mockStatus      int
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
			mockClient := newEndpointMock(t, "/beans", respondWith(tt.mockStatus, tt.mockResponse))

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
