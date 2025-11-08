package com.example.testapp;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
public class TestScheduledTasks {

    private static final Logger logger = LoggerFactory.getLogger(TestScheduledTasks.class);

    @Scheduled(cron = "0 * * * * *")
    public void cronTask() {
        logger.info("Executing cron task");
    }

    @Scheduled(fixedDelay = 300000) // 5 minutes
    public void fixedDelayTask() {
        logger.info("Executing fixed delay task");
    }

    @Scheduled(fixedRate = 1800000) // 30 minutes
    public void fixedRateTask() {
        logger.info("Executing fixed rate task");
    }

    @Scheduled(fixedDelay = 43200000, initialDelay = 900000) // 12h delay, 15m initial
    public void fixedDelayWithInitialDelayTask() {
        logger.info("Executing fixed delay with initial delay task");
    }
}
