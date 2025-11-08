package com.example.testapp;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class TestActuatorApplication {

    public static void main(String[] args) {
        SpringApplication.run(TestActuatorApplication.class, args);
    }
}
