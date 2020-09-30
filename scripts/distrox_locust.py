from locust import HttpUser, TaskSet, task
import random


class DistroxApiUser(HttpUser):
    min_wait = 100
    max_wait = 1000
    host = "http://localhost:8080/v1/kv"

    @task(1)
    class DistroxServerTasks(TaskSet):
        keys = []
        v = "\x68\x65\x6C\x6C\x6F\x20\x77\x6F\x72\x6C\x64\x21"
        entry_count = 300

        def put_entry(self, key):
            self.client.headers['Content-Type'] = "application/json"
            response = self.client.put("/{0}".format(key), data=self.v)
            if response.status_code == 201:
                self.keys.append(key)

        @task
        def get_entry(self):
            response = self.client.get("/{0}".format(random.choice(self.keys)))
            assert response.status_code == 200, "expected OK"

        def on_start(self):
            for i in range(self.entry_count):
                self.put_entry("key-{0}".format(random.randint(1, 1000)))
