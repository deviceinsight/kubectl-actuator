package actuator

func (c *actuatorClient) GetBeans() (*BeansResponse, error) {
	var beansResponse BeansResponse
	if err := c.getAndParse("/beans", "beans", "failed to get beans", &beansResponse); err != nil {
		return nil, err
	}
	return &beansResponse, nil
}

type BeansResponse struct {
	Contexts map[string]BeanContext `json:"contexts"`
}

type BeanContext struct {
	Beans  map[string]Bean `json:"beans"`
	Parent string          `json:"parent,omitempty"`
}

type Bean struct {
	Aliases      []string `json:"aliases"`
	Scope        string   `json:"scope"`
	Type         string   `json:"type"`
	Resource     string   `json:"resource,omitempty"`
	Dependencies []string `json:"dependencies"`
}
