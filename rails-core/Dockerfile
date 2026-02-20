FROM ruby:3.3-alpine

RUN apk add --no-cache build-base postgresql-dev tzdata git yaml-dev

WORKDIR /app
COPY Gemfile* ./
RUN bundle install
COPY . .

ENV RAILS_ENV=production
ENV RACK_ENV=production

EXPOSE 3000
CMD ["bundle", "exec", "puma", "-C", "config/puma.rb"]
